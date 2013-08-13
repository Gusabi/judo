// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package statecmd_test

import (
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/statecmd"
)

type ExposeSuite struct {
	testing.JujuConnSuite
}

var _ = Suite(&ExposeSuite{})

var serviceExposeTests = []struct {
	about   string
	service string
	err     string
	exposed bool
}{
	{
		about:   "unknown service name",
		service: "unknown-service",
		err:     `service "unknown-service" not found`,
	},
	{
		about:   "expose a service",
		service: "dummy-service",
		exposed: true,
	},
	{
		about:   "expose an already exposed service",
		service: "exposed-service",
		exposed: true,
	},
}

func (s *ExposeSuite) TestServiceExpose(c *C) {
	charm := s.AddTestingCharm(c, "dummy")
	serviceNames := []string{"dummy-service", "exposed-service"}
	svcs := make([]*state.Service, len(serviceNames))
	var err error
	for i, name := range serviceNames {
		svcs[i], err = s.State.AddService(name, charm)
		c.Assert(err, IsNil)
		c.Assert(svcs[i].IsExposed(), Equals, false)
	}
	err = svcs[1].SetExposed()
	c.Assert(err, IsNil)
	c.Assert(svcs[1].IsExposed(), Equals, true)

	for i, t := range serviceExposeTests {
		c.Logf("test %d. %s", i, t.about)
		err = statecmd.ServiceExpose(s.State, params.ServiceExpose{
			ServiceName: t.service,
		})
		if t.err != "" {
			c.Assert(err, ErrorMatches, t.err)
		} else {
			c.Assert(err, IsNil)
			service, err := s.State.Service(t.service)
			c.Assert(err, IsNil)
			c.Assert(service.IsExposed(), Equals, t.exposed)
		}
	}

	for _, s := range svcs {
		err = s.Destroy()
		c.Assert(err, IsNil)
	}
}
