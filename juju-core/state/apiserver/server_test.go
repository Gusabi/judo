// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver_test

import (
	"io"
	. "launchpad.net/gocheck"
	jujutesting "launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/rpc"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/apiserver"
	coretesting "launchpad.net/juju-core/testing"
	stdtesting "testing"
	"time"
)

func TestAll(t *stdtesting.T) {
	coretesting.MgoTestPackage(t)
}

var fastDialOpts = api.DialOpts{}

type serverSuite struct {
	jujutesting.JujuConnSuite
}

var _ = Suite(&serverSuite{})

func (s *serverSuite) TestStop(c *C) {
	// Start our own instance of the server so we have
	// a handle on it to stop it.
	srv, err := apiserver.NewServer(s.State, "localhost:0", []byte(coretesting.ServerCert), []byte(coretesting.ServerKey))
	c.Assert(err, IsNil)
	defer srv.Stop()

	stm, err := s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, IsNil)
	err = stm.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, IsNil)
	err = stm.SetPassword("password")
	c.Assert(err, IsNil)

	// Note we can't use openAs because we're not connecting to
	// s.APIConn.
	apiInfo := &api.Info{
		Tag:      stm.Tag(),
		Password: "password",
		Nonce:    "fake_nonce",
		Addrs:    []string{srv.Addr()},
		CACert:   []byte(coretesting.CACert),
	}
	st, err := api.Open(apiInfo, fastDialOpts)
	c.Assert(err, IsNil)
	defer st.Close()

	_, err = st.Machiner().Machine(stm.Tag())
	c.Assert(err, IsNil)

	err = srv.Stop()
	c.Assert(err, IsNil)

	_, err = st.Machiner().Machine(stm.Tag())
	// The client has not necessarily seen the server shutdown yet,
	// so there are two possible errors.
	if err != rpc.ErrShutdown && err != io.ErrUnexpectedEOF {
		c.Fatalf("unexpected error from request: %v", err)
	}

	// Check it can be stopped twice.
	err = srv.Stop()
	c.Assert(err, IsNil)
}

func (s *serverSuite) TestOpenAsMachineErrors(c *C) {
	stm, err := s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, IsNil)
	err = stm.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, IsNil)
	err = stm.SetPassword("password")
	c.Assert(err, IsNil)

	// This does almost exactly the same as OpenAPIAsMachine but checks
	// for failures instead.
	_, info, err := s.APIConn.Environ.StateInfo()
	info.Tag = stm.Tag()
	info.Password = "password"
	info.Nonce = "invalid-nonce"
	st, err := api.Open(info, fastDialOpts)
	c.Assert(err, ErrorMatches, params.CodeNotProvisioned)
	c.Assert(st, IsNil)

	// Try with empty nonce as well.
	info.Nonce = ""
	st, err = api.Open(info, fastDialOpts)
	c.Assert(err, ErrorMatches, params.CodeNotProvisioned)
	c.Assert(st, IsNil)

	// Finally, with the correct one succeeds.
	info.Nonce = "fake_nonce"
	st, err = api.Open(info, fastDialOpts)
	c.Assert(err, IsNil)
	c.Assert(st, NotNil)
	st.Close()

	// Now add another machine, intentionally unprovisioned.
	stm1, err := s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, IsNil)
	err = stm1.SetPassword("password")
	c.Assert(err, IsNil)

	// Try connecting, it will fail.
	info.Tag = stm1.Tag()
	info.Nonce = ""
	st, err = api.Open(info, fastDialOpts)
	c.Assert(err, ErrorMatches, params.CodeNotProvisioned)
	c.Assert(st, IsNil)
}

func (s *serverSuite) TestMachineLoginStartsPinger(c *C) {
	// Create a new machine to verify "agent alive" behavior.
	stm, err := s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, IsNil)
	err = stm.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, IsNil)
	err = stm.SetPassword("password")
	c.Assert(err, IsNil)

	// Not alive yet.
	s.State.Sync()
	alive, err := stm.AgentAlive()
	c.Assert(err, IsNil)
	c.Assert(alive, Equals, false)

	// Login as the machine agent of the created machine.
	st := s.OpenAPIAsMachine(c, stm.Tag(), "password", "fake_nonce")
	defer st.Close()

	// Make sure the pinger has started.
	s.State.Sync()
	stm.WaitAgentAlive(coretesting.LongWait)
	alive, err = stm.AgentAlive()
	c.Assert(err, IsNil)
	c.Assert(alive, Equals, true)

	// Now make sure it stops when connection is closed.
	c.Assert(st.Close(), IsNil)

	// Sync, then wait for a bit to make sure the state is updated.
	s.State.Sync()
	<-time.After(coretesting.ShortWait)
	s.State.Sync()

	c.Assert(err, IsNil)
	alive, err = stm.AgentAlive()
	c.Assert(err, IsNil)
	c.Assert(alive, Equals, false)
}
