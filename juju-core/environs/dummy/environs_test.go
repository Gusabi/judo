// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package dummy_test

import (
	. "launchpad.net/gocheck"
	_ "launchpad.net/juju-core/environs/dummy"
	"launchpad.net/juju-core/environs/jujutest"
	"launchpad.net/juju-core/testing"
	stdtesting "testing"
)

func init() {
	attrs := map[string]interface{}{
		"name":            "only",
		"type":            "dummy",
		"state-server":    true,
		"secret":          "pork",
		"admin-secret":    "fish",
		"authorized-keys": "foo",
		"ca-cert":         testing.CACert,
		"ca-private-key":  testing.CAKey,
	}
	Suite(&jujutest.LiveTests{
		TestConfig:     jujutest.TestConfig{attrs},
		CanOpenState:   true,
		HasProvisioner: false,
	})
	Suite(&jujutest.Tests{
		TestConfig: jujutest.TestConfig{attrs},
	})
}

func TestSuite(t *stdtesting.T) {
	testing.MgoTestPackage(t)
}
