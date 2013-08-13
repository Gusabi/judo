// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package statecmd_test

import (
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/statecmd"
)

type UnexposeSuite struct {
	testing.JujuConnSuite
}

var _ = Suite(&UnexposeSuite{})

var serviceUnexposeTests = []struct {
	about    string
	service  string
	err      string
	initial  bool
	expected bool
}{
	{
		about:   "unknown service name",
		service: "unknown-service",
		err:     `service "unknown-service" not found`,
	},
	{
		about:    "unexpose a service",
		service:  "dummy-service",
		initial:  true,
		expected: false,
	},
	{
		about:    "unexpose an already unexposed service",
		service:  "dummy-service",
		initial:  false,
		expected: false,
	},
}

func (s *UnexposeSuite) TestServiceUnexpose(c *C) {
	charm := s.AddTestingCharm(c, "dummy")
	for i, t := range serviceUnexposeTests {
		c.Logf("test %d. %s", i, t.about)
		svc, err := s.State.AddService("dummy-service", charm)
		c.Assert(err, IsNil)
		if t.initial {
			svc.SetExposed()
		}
		c.Assert(svc.IsExposed(), Equals, t.initial)
		params := params.ServiceUnexpose{ServiceName: t.service}
		err = statecmd.ServiceUnexpose(s.State, params)
		if t.err == "" {
			c.Assert(err, IsNil)
			svc.Refresh()
			c.Assert(svc.IsExposed(), Equals, t.expected)
		} else {
			c.Assert(err, ErrorMatches, t.err)
		}
		err = svc.Destroy()
		c.Assert(err, IsNil)
	}
}
