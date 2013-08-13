// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/downloader"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/utils"
	"os"
	"path"
)

// BundlesDir is responsible for storing and retrieving charm bundles
// identified by state charms.
type BundlesDir struct {
	path string
}

// NewBundlesDir returns a new BundlesDir which uses path for storage.
func NewBundlesDir(path string) *BundlesDir {
	return &BundlesDir{path}
}

// Read returns a charm bundle from the directory. If no bundle exists yet,
// one will be downloaded and validated and copied into the directory before
// being returned. Downloads will be aborted if a value is received on abort.
func (d *BundlesDir) Read(sch *state.Charm, abort <-chan struct{}) (*charm.Bundle, error) {
	path := d.bundlePath(sch)
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		} else if err = d.download(sch, abort); err != nil {
			return nil, err
		}
	}
	return charm.ReadBundle(path)
}

// download fetches the supplied charm and checks that it has the correct sha256
// hash, then copies it into the directory. If a value is received on abort, the
// download will be stopped.
func (d *BundlesDir) download(sch *state.Charm, abort <-chan struct{}) (err error) {
	defer utils.ErrorContextf(&err, "failed to download charm %q from %q", sch.URL(), sch.BundleURL())
	dir := d.downloadsPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	burl := sch.BundleURL().String()
	log.Infof("worker/uniter/charm: downloading %s from %s", sch.URL(), burl)
	dl := downloader.New(burl, dir)
	defer dl.Stop()
	for {
		select {
		case <-abort:
			log.Infof("worker/uniter/charm: download aborted")
			return fmt.Errorf("aborted")
		case st := <-dl.Done():
			if st.Err != nil {
				return st.Err
			}
			log.Infof("worker/uniter/charm: download complete")
			defer st.File.Close()
			hash := sha256.New()
			if _, err = io.Copy(hash, st.File); err != nil {
				return err
			}
			actualSha256 := hex.EncodeToString(hash.Sum(nil))
			if actualSha256 != sch.BundleSha256() {
				return fmt.Errorf(
					"expected sha256 %q, got %q", sch.BundleSha256(), actualSha256,
				)
			}
			log.Infof("worker/uniter/charm: download verified")
			if err := os.MkdirAll(d.path, 0755); err != nil {
				return err
			}
			return os.Rename(st.File.Name(), d.bundlePath(sch))
		}
	}
}

// bundlePath returns the path to the location where the verified charm
// bundle identified by sch will be, or has been, saved.
func (d *BundlesDir) bundlePath(sch *state.Charm) string {
	return path.Join(d.path, charm.Quote(sch.URL().String()))
}

// downloadsPath returns the path to the directory into which charms are
// downloaded.
func (d *BundlesDir) downloadsPath() string {
	return path.Join(d.path, "downloads")
}
