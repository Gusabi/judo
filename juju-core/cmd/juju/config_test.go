// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"bytes"
	"io/ioutil"

	. "launchpad.net/gocheck"
	"launchpad.net/goyaml"
	"launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/juju/testing"
	coretesting "launchpad.net/juju-core/testing"
)

// juju get and set tests (because one needs the other)

type ConfigSuite struct {
	testing.JujuConnSuite
}

var _ = Suite(&ConfigSuite{})

var getTests = []struct {
	service  string
	expected map[string]interface{}
}{
	{
		"dummy-service",
		map[string]interface{}{
			"service": "dummy-service",
			"charm":   "dummy",
			"settings": map[string]interface{}{
				"title": map[string]interface{}{
					"description": "A descriptive title used for the service.",
					"type":        "string",
					"value":       "Nearly There",
				},
				"skill-level": map[string]interface{}{
					"description": "A number indicating skill.",
					"type":        "int",
					"default":     true,
					"value":       nil,
				},
				"username": map[string]interface{}{
					"description": "The name of the initial account (given admin permissions).",
					"type":        "string",
					"value":       "admin001",
					"default":     true,
				},
				"outlook": map[string]interface{}{
					"description": "No default outlook.",
					"type":        "string",
					"default":     true,
					"value":       nil,
				},
			},
		},
	},

	// TODO(dfc) add additional services (need more charms)
	// TODO(dfc) add set tests
}

func (s *ConfigSuite) TestGetConfig(c *C) {
	sch := s.AddTestingCharm(c, "dummy")
	svc, err := s.State.AddService("dummy-service", sch)
	c.Assert(err, IsNil)
	err = svc.UpdateConfigSettings(charm.Settings{"title": "Nearly There"})
	c.Assert(err, IsNil)
	for _, t := range getTests {
		ctx := coretesting.Context(c)
		code := cmd.Main(&GetCommand{}, ctx, []string{t.service})
		c.Check(code, Equals, 0)
		c.Assert(ctx.Stderr.(*bytes.Buffer).String(), Equals, "")
		// round trip via goyaml to avoid being sucked into a quagmire of
		// map[interface{}]interface{} vs map[string]interface{}. This is
		// also required if we add json support to this command.
		buf, err := goyaml.Marshal(t.expected)
		c.Assert(err, IsNil)
		expected := make(map[string]interface{})
		err = goyaml.Unmarshal(buf, &expected)
		c.Assert(err, IsNil)

		actual := make(map[string]interface{})
		err = goyaml.Unmarshal(ctx.Stdout.(*bytes.Buffer).Bytes(), &actual)
		c.Assert(err, IsNil)
		c.Assert(actual, DeepEquals, expected)
	}
}

var setTests = []struct {
	about  string
	args   []string       // command to be executed
	expect charm.Settings // resulting configuration of the dummy service.
	err    string         // error regex
}{{
	about: "invalid option",
	args:  []string{"foo", "bar"},
	err:   "error: invalid option: \"foo\"\n",
}, {
	about: "whack option",
	args:  []string{"=bar"},
	err:   "error: invalid option: \"=bar\"\n",
}, {
	about: "--config missing",
	args:  []string{"--config", "missing.yaml"},
	err:   "error.*no such file or directory\n",
}, {
	about: "set with options",
	args:  []string{"username=hello"},
	expect: charm.Settings{
		"username": "hello",
	},
}, {
	about: "set with option values containing =",
	args:  []string{"username=hello=foo"},
	expect: charm.Settings{
		"username": "hello=foo",
	},
}, {
	about: "--config $FILE test",
	args:  []string{"--config", "testconfig.yaml"},
	expect: charm.Settings{
		"username":    "admin001",
		"skill-level": int64(9000), // charm int types are int64
	},
},
}

func (s *ConfigSuite) TestSetConfig(c *C) {
	sch := s.AddTestingCharm(c, "dummy")
	svc, err := s.State.AddService("dummy-service", sch)
	c.Assert(err, IsNil)
	dir := c.MkDir()
	setupConfigfile(c, dir)
	for i, t := range setTests {
		args := append([]string{"dummy-service"}, t.args...)
		c.Logf("test %d. %s", i, t.about)
		ctx := coretesting.ContextForDir(c, dir)
		code := cmd.Main(&SetCommand{}, ctx, args)
		if t.err != "" {
			c.Check(code, Not(Equals), 0)
			c.Assert(ctx.Stderr.(*bytes.Buffer).String(), Matches, t.err)
		} else {
			c.Check(code, Equals, 0)
			settings, err := svc.ConfigSettings()
			c.Assert(err, IsNil)
			c.Assert(settings, DeepEquals, t.expect)
		}
	}
}

func setupConfigfile(c *C, dir string) string {
	ctx := coretesting.ContextForDir(c, dir)
	path := ctx.AbsPath("testconfig.yaml")
	content := []byte("dummy-service:\n  skill-level: 9000\n  username: admin001\n\n")
	err := ioutil.WriteFile(path, content, 0666)
	c.Assert(err, IsNil)
	return path
}
