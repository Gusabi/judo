// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package agent_test

import (
	"fmt"
	stdtesting "testing"

	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/errors"
	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"launchpad.net/juju-core/state/api/params"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/testing/checkers"
)

func TestAll(t *stdtesting.T) {
	coretesting.MgoTestPackage(t)
}

type machineSuite struct {
	testing.JujuConnSuite
	machine *state.Machine
	st      *api.State
}

var _ = gc.Suite(&machineSuite{})

func (s *machineSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	// Create a machine so we can log in as its agent.
	var err error
	s.machine, err = s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, gc.IsNil)
	err = s.machine.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, gc.IsNil)
	err = s.machine.SetPassword("machine-password")

	s.st = s.OpenAPIAsMachine(c, s.machine.Tag(), "machine-password", "fake_nonce")
}

func (s *machineSuite) TearDownTest(c *gc.C) {
	if s.st != nil {
		c.Assert(s.st.Close(), gc.IsNil)
	}
	s.JujuConnSuite.TearDownTest(c)
}

func (s *machineSuite) TestMachineEntity(c *gc.C) {
	m, err := s.st.Agent().Entity("42")
	c.Assert(err, gc.ErrorMatches, "permission denied")
	c.Assert(params.ErrCode(err), gc.Equals, params.CodeUnauthorized)
	c.Assert(m, gc.IsNil)

	m, err = s.st.Agent().Entity(s.machine.Tag())
	c.Assert(err, gc.IsNil)
	c.Assert(m.Tag(), gc.Equals, s.machine.Tag())
	c.Assert(m.Life(), gc.Equals, params.Alive)
	c.Assert(m.Jobs(), gc.DeepEquals, []params.MachineJob{params.JobHostUnits})

	err = s.machine.EnsureDead()
	c.Assert(err, gc.IsNil)
	err = s.machine.Remove()
	c.Assert(err, gc.IsNil)

	m, err = s.st.Agent().Entity(s.machine.Tag())
	c.Assert(err, gc.ErrorMatches, fmt.Sprintf("machine %s not found", s.machine.Id()))
	c.Assert(params.ErrCode(err), gc.Equals, params.CodeNotFound)
	c.Assert(m, gc.IsNil)
}

func (s *machineSuite) TestEntitySetPassword(c *gc.C) {
	entity, err := s.st.Agent().Entity(s.machine.Tag())
	c.Assert(err, gc.IsNil)

	err = entity.SetPassword("foo")
	c.Assert(err, gc.IsNil)

	err = s.machine.Refresh()
	c.Assert(err, gc.IsNil)
	c.Assert(s.machine.PasswordValid("bar"), gc.Equals, false)
	c.Assert(s.machine.PasswordValid("foo"), gc.Equals, true)

	// Check that we cannot log in to mongo with the wrong password.
	info := s.StateInfo(c)
	info.Tag = entity.Tag()
	info.Password = "bar"
	err = tryOpenState(info)
	c.Assert(err, checkers.Satisfies, errors.IsUnauthorizedError)

	// Check that we can log in with the correct password
	info.Password = "foo"
	st, err := state.Open(info, state.DialOpts{})
	c.Assert(err, gc.IsNil)
	st.Close()
}

func tryOpenState(info *state.Info) error {
	st, err := state.Open(info, state.DialOpts{})
	if err == nil {
		st.Close()
	}
	return err
}
