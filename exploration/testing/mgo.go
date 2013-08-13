// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	stdtesting "testing"
	"time"

	"labix.org/v2/mgo"
	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/cert"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/utils"
)

var (
	// MgoAddr holds the address of the shared MongoDB server set up by
	// MgoTestPackage.
	MgoAddr string

	// mgoServer holds the running MongoDB command.
	mgoServer *exec.Cmd

	// mgoExited receives a value when the mongodb server exits.
	mgoExited <-chan struct{}

	// mgoDir holds the directory that MongoDB is running in.
	mgoDir string
)

// We specify a timeout to mgo.Dial, to prevent
// mongod failures hanging the tests.
const mgoDialTimeout = 5 * time.Second

// MgoSuite is a suite that deletes all content from the shared MongoDB
// server at the end of every test and supplies a connection to the shared
// MongoDB server.
type MgoSuite struct {
	Session *mgo.Session
}

// startMgoServer starts a MongoDB server in a temporary directory.
// It panics if it encounters an error.
func startMgoServer() error {
	dbdir, err := ioutil.TempDir("", "test-mgo")
	if err != nil {
		return err
	}
	pemPath := filepath.Join(dbdir, "server.pem")
	err = ioutil.WriteFile(pemPath, []byte(ServerCert+ServerKey), 0600)
	if err != nil {
		return fmt.Errorf("cannot write cert/key PEM: %v", err)
	}
	mgoport := strconv.Itoa(FindTCPPort())
	mgoargs := []string{
		"--auth",
		"--dbpath", dbdir,
		"--sslOnNormalPorts",
		"--sslPEMKeyFile", pemPath,
		"--sslPEMKeyPassword", "ignored",
		"--bind_ip", "localhost",
		"--port", mgoport,
		"--nssize", "1",
		"--noprealloc",
		"--smallfiles",
		"--nojournal",
	}
	server := exec.Command("mongod", mgoargs...)
	out, err := server.StdoutPipe()
	if err != nil {
		return err
	}
	server.Stderr = server.Stdout
	exited := make(chan struct{})
	go func() {
		lines := readLines(out, 20)
		err := server.Wait()
		exitErr, _ := err.(*exec.ExitError)
		if err == nil || exitErr != nil && exitErr.Exited() {
			// mongodb has exited without being killed, so print the
			// last few lines of its log output.
			for _, line := range lines {
				log.Infof("mongod: %s", line)
			}
		}
		close(exited)
	}()
	mgoExited = exited
	if err := server.Start(); err != nil {
		os.RemoveAll(dbdir)
		return err
	}
	MgoAddr = "localhost:" + mgoport
	mgoServer = server
	mgoDir = dbdir
	return nil
}

func destroyMgoServer() {
	if mgoServer != nil {
		mgoServer.Process.Kill()
		<-mgoExited
		os.RemoveAll(mgoDir)
		MgoAddr, mgoServer, mgoExited, mgoDir = "", nil, nil, ""
	}
}

// MgoTestPackage should be called to register the tests for any package that
// requires a MongoDB server.
func MgoTestPackage(t *stdtesting.T) {
	if err := startMgoServer(); err != nil {
		t.Fatal(err)
	}
	defer destroyMgoServer()
	TestingT(t)
}

func (s *MgoSuite) SetUpSuite(c *C) {
	if MgoAddr == "" {
		panic("MgoSuite tests must be run with MgoTestPackage")
	}
	mgo.SetStats(true)
	// Make tests that use password authentication faster.
	utils.FastInsecureHash = true
}

// readLines reads lines from the given reader and returns
// the last n non-empty lines, ignoring empty lines.
func readLines(r io.Reader, n int) []string {
	br := bufio.NewReader(r)
	lines := make([]string, n)
	i := 0
	for {
		line, err := br.ReadString('\n')
		if line = strings.TrimRight(line, "\n"); line != "" {
			lines[i%n] = line
			i++
		}
		if err != nil {
			break
		}
	}
	final := make([]string, 0, n+1)
	if i > n {
		final = append(final, fmt.Sprintf("[%d lines omitted]", i-n))
	}
	for j := 0; j < n; j++ {
		if line := lines[(j+i)%n]; line != "" {
			final = append(final, line)
		}
	}
	return final
}

func (s *MgoSuite) TearDownSuite(c *C) {
	utils.FastInsecureHash = false
}

// MgoDial returns a new connection to the shared MongoDB server.
func MgoDial() *mgo.Session {
	pool := x509.NewCertPool()
	xcert, err := cert.ParseCert([]byte(CACert))
	if err != nil {
		panic(err)
	}
	pool.AddCert(xcert)
	tlsConfig := &tls.Config{
		RootCAs:    pool,
		ServerName: "anything",
	}
	session, err := mgo.DialWithInfo(&mgo.DialInfo{
		Addrs: []string{MgoAddr},
		Dial: func(addr net.Addr) (net.Conn, error) {
			return tls.Dial("tcp", addr.String(), tlsConfig)
		},
		Timeout: mgoDialTimeout,
	})
	if err != nil {
		panic(err)
	}
	return session
}

func (s *MgoSuite) SetUpTest(c *C) {
	mgo.ResetStats()
	s.Session = MgoDial()
}

// MgoReset deletes all content from the shared MongoDB server.
func MgoReset() {
	session := MgoDial()
	defer session.Close()
	dbnames, err := session.DatabaseNames()
	if isUnauthorized(err) {
		// If we've got an unauthorized access error, we're
		// locked out of the database.  We restart it to regain
		// access.  This should only happen when tests fail.
		destroyMgoServer()
		log.Noticef("testing: restarting MongoDB server after unauthorized access")
		if err := startMgoServer(); err != nil {
			panic(err)
		}
		return
	}
	if err != nil {
		panic(err)
	}
	for _, name := range dbnames {
		switch name {
		case "admin", "local", "config":
		default:
			err = session.DB(name).DropDatabase()
			if err != nil {
				panic(fmt.Errorf("Cannot drop MongoDB database %v: %v", name, err))
			}
		}
	}
}

func isUnauthorized(err error) bool {
	if err, ok := err.(*mgo.QueryError); ok {
		if err.Code == 10057 || err.Message == "need to login" {
			return true
		}
	}
	return false
}

func (s *MgoSuite) TearDownTest(c *C) {
	MgoReset()
	s.Session.Close()
	for i := 0; ; i++ {
		stats := mgo.GetStats()
		if stats.SocketsInUse == 0 && stats.SocketsAlive == 0 {
			break
		}
		if i == 20 {
			c.Fatal("Test left sockets in a dirty state")
		}
		c.Logf("Waiting for sockets to die: %d in use, %d alive", stats.SocketsInUse, stats.SocketsAlive)
		time.Sleep(500 * time.Millisecond)
	}
}

// FindTCPPort finds an unused TCP port and returns it.
// Use of this function has an inherent race condition - another
// process may claim the port before we try to use it.
// We hope that the probability is small enough during
// testing to be negligible.
func FindTCPPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
