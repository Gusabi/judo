// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package localstorage_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/localstorage"
	"launchpad.net/juju-core/errors"
	jc "launchpad.net/juju-core/testing/checkers"
)

type storageSuite struct{}

var _ = Suite(&storageSuite{})

// TestPersistence tests the adding, reading, listing and removing
// of files from the local storage.
func (s *storageSuite) TestPersistence(c *C) {
	listener, _, _ := startServer(c)
	defer listener.Close()

	storage := localstorage.Client(listener.Addr().String())
	names := []string{
		"aa",
		"zzz/aa",
		"zzz/bb",
	}
	for _, name := range names {
		checkFileDoesNotExist(c, storage, name)
		checkPutFile(c, storage, name, []byte(name))
	}
	checkList(c, storage, "", names)
	checkList(c, storage, "a", []string{"aa"})
	checkList(c, storage, "zzz/", []string{"zzz/aa", "zzz/bb"})

	storage2 := localstorage.Client(listener.Addr().String())
	for _, name := range names {
		checkFileHasContents(c, storage2, name, []byte(name))
	}

	// remove the first file and check that the others remain.
	err := storage2.Remove(names[0])
	c.Check(err, IsNil)

	// check that it's ok to remove a file twice.
	err = storage2.Remove(names[0])
	c.Check(err, IsNil)

	// ... and check it's been removed in the other environment
	checkFileDoesNotExist(c, storage, names[0])

	// ... and that the rest of the files are still around
	checkList(c, storage2, "", names[1:])

	for _, name := range names[1:] {
		err := storage2.Remove(name)
		c.Assert(err, IsNil)
	}

	// check they've all gone
	checkList(c, storage2, "", nil)

	// Check that RemoveAll works.
	checkRemoveAll(c, storage2)
}

func checkList(c *C, storage environs.StorageReader, prefix string, names []string) {
	lnames, err := storage.List(prefix)
	c.Assert(err, IsNil)
	c.Assert(lnames, DeepEquals, names)
}

type readerWithClose struct {
	*bytes.Buffer
	closeCalled bool
}

var _ io.Reader = (*readerWithClose)(nil)
var _ io.Closer = (*readerWithClose)(nil)

func (r *readerWithClose) Close() error {
	r.closeCalled = true
	return nil
}

func checkPutFile(c *C, storage environs.StorageWriter, name string, contents []byte) {
	c.Logf("check putting file %s ...", name)
	reader := &readerWithClose{bytes.NewBuffer(contents), false}
	err := storage.Put(name, reader, int64(len(contents)))
	c.Assert(err, IsNil)
	c.Assert(reader.closeCalled, jc.IsFalse)
}

func checkFileDoesNotExist(c *C, storage environs.StorageReader, name string) {
	var notFoundError *errors.NotFoundError
	r, err := storage.Get(name)
	c.Assert(r, IsNil)
	c.Assert(err, FitsTypeOf, notFoundError)
}

func checkFileHasContents(c *C, storage environs.StorageReader, name string, contents []byte) {
	r, err := storage.Get(name)
	c.Assert(err, IsNil)
	c.Check(r, NotNil)
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	c.Check(err, IsNil)
	c.Check(data, DeepEquals, contents)

	url, err := storage.URL(name)
	c.Assert(err, IsNil)

	resp, err := http.Get(url)
	c.Assert(err, IsNil)
	data, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, http.StatusOK, Commentf("error response: %s", data))
	c.Check(data, DeepEquals, contents)
}

func checkRemoveAll(c *C, storage environs.Storage) {
	contents := []byte("File contents.")
	aFile := "a-file.txt"
	err := storage.Put(aFile, bytes.NewBuffer(contents), int64(len(contents)))
	c.Assert(err, IsNil)
	err = storage.Put("empty-file", bytes.NewBuffer(nil), 0)
	c.Assert(err, IsNil)

	err = storage.RemoveAll()
	c.Assert(err, IsNil)

	files, err := storage.List("")
	c.Assert(err, IsNil)
	c.Check(files, HasLen, 0)

	_, err = storage.Get(aFile)
	c.Assert(err, NotNil)
	c.Check(err, ErrorMatches, fmt.Sprintf("file %q not found", aFile))
}
