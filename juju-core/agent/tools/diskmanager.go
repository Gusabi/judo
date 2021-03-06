// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package tools

import (
	"io"

	"launchpad.net/juju-core/version"
)

// DiskManager keeps track of a collections of Juju agent tools in a directory
// structure on disk.
type DiskManager struct {
	dataDir string
}

// NewDiskManager returns a DiskManager handling a given directory.
// *DiskManager conforms to the ToolsManager interface
func NewDiskManager(dataDir string) *DiskManager {
	return &DiskManager{dataDir: dataDir}
}

func (d *DiskManager) ReadTools(vers version.Binary) (*Tools, error) {
	return ReadTools(d.dataDir, vers)
}

func (d *DiskManager) UnpackTools(tools *Tools, r io.Reader) error {
	return UnpackTools(d.dataDir, tools, r)
}

func (d *DiskManager) SharedToolsDir(vers version.Binary) string {
	return SharedToolsDir(d.dataDir, vers)
}
