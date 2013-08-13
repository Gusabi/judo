// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package maas

import (
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/version"
)

type ConfigSuite struct{}

var _ = gc.Suite(&ConfigSuite{})

// copyAttrs copies values from src into dest.  If src contains a key that was
// already in dest, its value in dest will still be updated to the one from
// src.
func copyAttrs(src, dest map[string]interface{}) {
	for k, v := range src {
		dest[k] = v
	}
}

// newConfig creates a MAAS environment config from attributes.
func newConfig(values map[string]interface{}) (*maasEnvironConfig, error) {
	defaults := map[string]interface{}{
		"name":            "testenv",
		"type":            "maas",
		"admin-secret":    "ssshhhhhh",
		"authorized-keys": "I-am-not-a-real-key",
		"agent-version":   version.CurrentNumber().String(),
		// These are not needed by MAAS, but juju-core breaks without them. Needs
		// fixing there.
		"ca-cert":        testing.CACert,
		"ca-private-key": testing.CAKey,
	}
	cfg := make(map[string]interface{})
	copyAttrs(defaults, cfg)
	copyAttrs(values, cfg)
	env, err := environs.NewFromAttrs(cfg)
	if err != nil {
		return nil, err
	}
	return env.(*maasEnviron).ecfg(), nil
}

func (*ConfigSuite) TestParsesMAASSettings(c *gc.C) {
	server := "http://maas.testing.invalid/maas/"
	oauth := "consumer-key:resource-token:resource-secret"
	future := "futurama"
	ecfg, err := newConfig(map[string]interface{}{
		"maas-server": server,
		"maas-oauth":  oauth,
		"future-key":  future,
	})
	c.Assert(err, gc.IsNil)
	c.Check(ecfg.MAASServer(), gc.Equals, server)
	c.Check(ecfg.MAASOAuth(), gc.DeepEquals, oauth)
	c.Check(ecfg.UnknownAttrs()["future-key"], gc.DeepEquals, future)
}

func (*ConfigSuite) TestChecksWellFormedMaasServer(c *gc.C) {
	_, err := newConfig(map[string]interface{}{
		"maas-server": "This should have been a URL.",
		"maas-oauth":  "consumer-key:resource-token:resource-secret",
	})
	c.Assert(err, gc.NotNil)
	c.Check(err, gc.ErrorMatches, ".*malformed maas-server.*")
}

func (*ConfigSuite) TestChecksWellFormedMaasOAuth(c *gc.C) {
	_, err := newConfig(map[string]interface{}{
		"maas-server": "http://maas.testing.invalid/maas/",
		"maas-oauth":  "This should have been a 3-part token.",
	})
	c.Assert(err, gc.NotNil)
	c.Check(err, gc.ErrorMatches, ".*malformed maas-oauth.*")
}

func (*ConfigSuite) TestValidateUpcallsEnvironsConfigValidate(c *gc.C) {
	// The base Validate() function will not allow an environment to
	// change its name.  Trigger that error so as to prove that the
	// environment provider's Validate() calls the base Validate().
	baseAttrs := map[string]interface{}{
		"maas-server": "http://maas.testing.invalid/maas/",
		"maas-oauth":  "consumer-key:resource-token:resource-secret",
	}
	oldCfg, err := newConfig(baseAttrs)
	c.Assert(err, gc.IsNil)
	newName := oldCfg.Name() + "-but-different"
	newCfg, err := oldCfg.Apply(map[string]interface{}{"name": newName})
	c.Assert(err, gc.IsNil)

	_, err = maasEnvironProvider{}.Validate(newCfg, oldCfg.Config)

	c.Assert(err, gc.NotNil)
	c.Check(err, gc.ErrorMatches, ".*cannot change name.*")
}
