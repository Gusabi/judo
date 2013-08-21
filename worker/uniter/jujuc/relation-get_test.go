// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc_test

import (
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/worker/uniter/jujuc"
	"path/filepath"
)

type RelationGetSuite struct {
	ContextSuite
}

var _ = Suite(&RelationGetSuite{})

func (s *RelationGetSuite) SetUpTest(c *C) {
	s.ContextSuite.SetUpTest(c)
	s.rels[0].units["u/0"]["private-address"] = "foo: bar\n"
	s.rels[1].units["m/0"] = map[string]interface{}{"pew": "pew\npew\n"}
	s.rels[1].units["u/1"] = map[string]interface{}{"value": "12345"}
}

var relationGetTests = []struct {
	summary  string
	relid    int
	unit     string
	args     []string
	code     int
	out      string
	checkctx func(*C, *cmd.Context)
}{
	{
		summary: "no default relation",
		relid:   -1,
		code:    2,
		out:     `no relation id specified`,
	}, {
		summary: "explicit relation, not known",
		relid:   -1,
		code:    2,
		args:    []string{"-r", "burble:123"},
		out:     `invalid value "burble:123" for flag -r: unknown relation id`,
	}, {
		summary: "default relation, no unit chosen",
		relid:   1,
		code:    2,
		out:     `no unit id specified`,
	}, {
		summary: "explicit relation, no unit chosen",
		relid:   -1,
		code:    2,
		args:    []string{"-r", "burble:1"},
		out:     `no unit id specified`,
	}, {
		summary: "missing key",
		relid:   1,
		unit:    "m/0",
		args:    []string{"ker-plunk"},
	}, {
		summary: "missing unit",
		relid:   1,
		unit:    "bad/0",
		code:    1,
		out:     `unknown unit bad/0`,
	}, {
		summary: "all keys with implicit member",
		relid:   1,
		unit:    "m/0",
		out:     "pew: 'pew\n\n  pew\n\n'",
	}, {
		summary: "all keys with explicit member",
		relid:   1,
		args:    []string{"-", "m/0"},
		out:     "pew: 'pew\n\n  pew\n\n'",
	}, {
		summary: "all keys with explicit non-member",
		relid:   1,
		args:    []string{"-", "u/1"},
		out:     `value: "12345"`,
	}, {
		summary: "all keys with explicit local",
		relid:   0,
		args:    []string{"-", "u/0"},
		out:     "private-address: 'foo: bar\n\n'",
	}, {
		summary: "specific key with implicit member",
		relid:   1,
		unit:    "m/0",
		args:    []string{"pew"},
		out:     "pew\npew\n",
	}, {
		summary: "specific key with explicit member",
		relid:   1,
		args:    []string{"pew", "m/0"},
		out:     "pew\npew\n",
	}, {
		summary: "specific key with explicit non-member",
		relid:   1,
		args:    []string{"value", "u/1"},
		out:     "12345",
	}, {
		summary: "specific key with explicit local",
		relid:   0,
		args:    []string{"private-address", "u/0"},
		out:     "foo: bar\n",
	}, {
		summary: "explicit smart formatting 1",
		relid:   1,
		unit:    "m/0",
		args:    []string{"--format", "smart"},
		out:     "pew: 'pew\n\n  pew\n\n'",
	}, {
		summary: "explicit smart formatting 2",
		relid:   1,
		unit:    "m/0",
		args:    []string{"pew", "--format", "smart"},
		out:     "pew\npew\n",
	}, {
		summary: "explicit smart formatting 3",
		relid:   1,
		args:    []string{"value", "u/1"},
		out:     "12345",
	}, {
		summary: "json formatting 1",
		relid:   1,
		unit:    "m/0",
		args:    []string{"--format", "json"},
		out:     `{"pew":"pew\npew\n"}`,
	}, {
		summary: "json formatting 2",
		relid:   1,
		unit:    "m/0",
		args:    []string{"pew", "--format", "json"},
		out:     `"pew\npew\n"`,
	}, {
		summary: "json formatting 3",
		relid:   1,
		args:    []string{"value", "u/1", "--format", "json"},
		out:     `"12345"`,
	}, {
		summary: "yaml formatting 1",
		relid:   1,
		unit:    "m/0",
		args:    []string{"--format", "yaml"},
		out:     "pew: 'pew\n\n  pew\n\n'",
	}, {
		summary: "yaml formatting 2",
		relid:   1,
		unit:    "m/0",
		args:    []string{"pew", "--format", "yaml"},
		out:     "'pew\n\n  pew\n\n'",
	}, {
		summary: "yaml formatting 3",
		relid:   1,
		args:    []string{"value", "u/1", "--format", "yaml"},
		out:     `"12345"`,
	},
}

func (s *RelationGetSuite) TestRelationGet(c *C) {
	for i, t := range relationGetTests {
		c.Logf("test %d: %s", i, t.summary)
		hctx := s.GetHookContext(c, t.relid, t.unit)
		com, err := jujuc.NewCommand(hctx, "relation-get")
		c.Assert(err, IsNil)
		ctx := testing.Context(c)
		code := cmd.Main(com, ctx, t.args)
		c.Assert(code, Equals, t.code)
		if code == 0 {
			c.Assert(bufferString(ctx.Stderr), Equals, "")
			expect := t.out
			if expect != "" {
				expect = expect + "\n"
			}
			c.Assert(bufferString(ctx.Stdout), Equals, expect)
		} else {
			c.Assert(bufferString(ctx.Stdout), Equals, "")
			expect := fmt.Sprintf(`(.|\n)*error: %s\n`, t.out)
			c.Assert(bufferString(ctx.Stderr), Matches, expect)
		}
	}
}

var helpTemplate = `
usage: %s
purpose: get relation settings

options:
--format  (= smart)
    specify output format (json|smart|yaml)
-o, --output (= "")
    specify an output file
-r  (= %s)
    specify a relation by id

relation-get prints the value of a unit's relation setting, specified by key.
If no key is given, or if the key is "-", all keys and values will be printed.
%s`[1:]

var relationGetHelpTests = []struct {
	summary string
	relid   int
	unit    string
	usage   string
	rel     string
}{
	{
		summary: "no default relation",
		relid:   -1,
		usage:   "relation-get [options] <key> <unit id>",
	}, {
		summary: "no default unit",
		relid:   1,
		usage:   "relation-get [options] <key> <unit id>",
		rel:     "peer1:1",
	}, {
		summary: "default unit",
		relid:   1,
		unit:    "any/1",
		usage:   `relation-get [options] [<key> [<unit id>]]`,
		rel:     "peer1:1",
	},
}

func (s *RelationGetSuite) TestHelp(c *C) {
	for i, t := range relationGetHelpTests {
		c.Logf("test %d", i)
		hctx := s.GetHookContext(c, t.relid, t.unit)
		com, err := jujuc.NewCommand(hctx, "relation-get")
		c.Assert(err, IsNil)
		ctx := testing.Context(c)
		code := cmd.Main(com, ctx, []string{"--help"})
		c.Assert(code, Equals, 0)
		unitHelp := ""
		if t.unit != "" {
			unitHelp = fmt.Sprintf("Current default unit id is %q.\n", t.unit)
		}
		expect := fmt.Sprintf(helpTemplate, t.usage, t.rel, unitHelp)
		c.Assert(bufferString(ctx.Stdout), Equals, expect)
		c.Assert(bufferString(ctx.Stderr), Equals, "")
	}
}

func (s *RelationGetSuite) TestOutputPath(c *C) {
	hctx := s.GetHookContext(c, 1, "m/0")
	com, err := jujuc.NewCommand(hctx, "relation-get")
	c.Assert(err, IsNil)
	ctx := testing.Context(c)
	code := cmd.Main(com, ctx, []string{"--output", "some-file", "pew"})
	c.Assert(code, Equals, 0)
	c.Assert(bufferString(ctx.Stderr), Equals, "")
	c.Assert(bufferString(ctx.Stdout), Equals, "")
	content, err := ioutil.ReadFile(filepath.Join(ctx.Dir, "some-file"))
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "pew\npew\n\n")
}
