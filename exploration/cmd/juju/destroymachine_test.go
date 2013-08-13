// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	. "launchpad.net/gocheck"
	jujutesting "launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/testing"
)

type DestroyMachineSuite struct {
	jujutesting.RepoSuite
}

var _ = Suite(&DestroyMachineSuite{})

func runDestroyMachine(c *C, args ...string) error {
	_, err := testing.RunCommand(c, &DestroyMachineCommand{}, args)
	return err
}

func (s *DestroyMachineSuite) TestDestroyMachine(c *C) {
	// Create a machine running a unit.
	testing.Charms.BundlePath(s.SeriesPath, "riak")
	err := runDeploy(c, "local:riak", "riak")
	c.Assert(err, IsNil)

	// Get the state entities to allow sane testing.
	u, err := s.State.Unit("riak/0")
	c.Assert(err, IsNil)
	mid, err := u.AssignedMachineId()
	c.Assert(err, IsNil)
	c.Assert(mid, Equals, "0")

	// Try to destroy the machine and fail.
	err = runDestroyMachine(c, "0")
	c.Assert(err, ErrorMatches, `no machines were destroyed: machine 0 has unit "riak/0" assigned`)

	// Remove the unit, and try to destroy the machine along with another that
	// doesn't exist; check that the machine is destroyed, but the missing one
	// is warned about.
	err = u.Destroy()
	c.Assert(err, IsNil)
	err = u.EnsureDead()
	c.Assert(err, IsNil)
	err = u.Remove()
	c.Assert(err, IsNil)
	err = runDestroyMachine(c, "0", "1")
	c.Assert(err, ErrorMatches, `some machines were not destroyed: machine 1 does not exist`)
	m0, err := s.State.Machine("0")
	c.Assert(err, IsNil)
	c.Assert(m0.Life(), Equals, state.Dying)

	// Destroying a destroyed machine is a no-op.
	err = runDestroyMachine(c, "0")
	c.Assert(err, IsNil)
	err = m0.Refresh()
	c.Assert(err, IsNil)
	c.Assert(m0.Life(), Equals, state.Dying)

	// As is destroying a Dead machine; destroying it alongside a JobManageEnviron
	// machine complains only about the JMA machine.
	err = m0.EnsureDead()
	c.Assert(err, IsNil)
	m1, err := s.State.AddMachine("series", state.JobManageEnviron)
	c.Assert(err, IsNil)
	err = runDestroyMachine(c, "0", "1")
	c.Assert(err, ErrorMatches, `some machines were not destroyed: machine 1 is required by the environment`)
	err = m0.Refresh()
	c.Assert(err, IsNil)
	c.Assert(m0.Life(), Equals, state.Dead)
	err = m1.Refresh()
	c.Assert(err, IsNil)
	c.Assert(m1.Life(), Equals, state.Alive)

	// Check invalid args.
	err = runDestroyMachine(c)
	c.Assert(err, ErrorMatches, `no machines specified`)
	err = runDestroyMachine(c, "1", "2", "nonsense", "rubbish")
	c.Assert(err, ErrorMatches, `invalid machine id "nonsense"`)
}
