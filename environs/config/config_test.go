// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package config_test

import (
	stdtesting "testing"
	"time"

	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/cert"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/schema"
	"launchpad.net/juju-core/testing"
	jc "launchpad.net/juju-core/testing/checkers"
	"launchpad.net/juju-core/version"
)

func Test(t *stdtesting.T) {
	gc.TestingT(t)
}

type ConfigSuite struct {
	testing.LoggingSuite
	home string
}

var _ = gc.Suite(&ConfigSuite{})

type attrs map[string]interface{}

type configTest struct {
	about string
	attrs map[string]interface{}
	err   string
}

var configTests = []configTest{
	{
		about: "The minimum good configuration",
		attrs: attrs{
			"type": "my-type",
			"name": "my-name",
		},
	}, {
		about: "Metadata URLs",
		attrs: attrs{
			"type":               "my-type",
			"name":               "my-name",
			"image-metadata-url": "image-url",
			"tools-url":          "tools-url",
		},
	}, {
		about: "Explicit series",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"default-series": "my-series",
		},
	}, {
		about: "Implicit series with empty value",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"default-series": "",
		},
	}, {
		about: "Explicit authorized-keys",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
		},
	}, {
		about: "Load authorized-keys from path",
		attrs: attrs{
			"type":                 "my-type",
			"name":                 "my-name",
			"authorized-keys-path": "~/.ssh/authorized_keys2",
		},
	}, {
		about: "CA cert & key from path",
		attrs: attrs{
			"type":                "my-type",
			"name":                "my-name",
			"ca-cert-path":        "cacert2.pem",
			"ca-private-key-path": "cakey2.pem",
		},
	}, {
		about: "CA cert & key from path; cert attribute set too",
		attrs: attrs{
			"type":                "my-type",
			"name":                "my-name",
			"ca-cert-path":        "cacert2.pem",
			"ca-cert":             "ignored",
			"ca-private-key-path": "cakey2.pem",
		},
	}, {
		about: "CA cert & key from ~ path",
		attrs: attrs{
			"type":                "my-type",
			"name":                "my-name",
			"ca-cert-path":        "~/othercert.pem",
			"ca-private-key-path": "~/otherkey.pem",
		},
	}, {
		about: "CA cert only from ~ path",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert-path":   "~/othercert.pem",
			"ca-private-key": "",
		},
	}, {
		about: "CA cert only as attribute",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        caCert,
			"ca-private-key": "",
		},
	}, {
		about: "CA cert and key as attributes",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        caCert,
			"ca-private-key": caKey,
		},
	}, {
		about: "Mismatched CA cert and key",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        caCert,
			"ca-private-key": caKey2,
		},
		err: "bad CA certificate/key in configuration: crypto/tls: private key does not match public key",
	}, {
		about: "Invalid CA cert",
		attrs: attrs{
			"type":    "my-type",
			"name":    "my-name",
			"ca-cert": invalidCACert,
		},
		err: `bad CA certificate/key in configuration: (asn1:|ASN\.1) syntax error:.*`,
	}, {
		about: "Invalid CA key",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        caCert,
			"ca-private-key": invalidCAKey,
		},
		err: "bad CA certificate/key in configuration: crypto/tls:.*",
	}, {
		about: "No CA cert or key",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        "",
			"ca-private-key": "",
		},
	}, {
		about: "CA key but no cert",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        "",
			"ca-private-key": caKey,
		},
		err: "bad CA certificate/key in configuration: crypto/tls:.*",
	}, {
		about: "No CA key",
		attrs: attrs{
			"type":           "my-type",
			"name":           "my-name",
			"ca-cert":        "foo",
			"ca-private-key": "",
		},
		err: "bad CA certificate/key in configuration: no certificates found",
	}, {
		about: "CA cert specified as non-existent file",
		attrs: attrs{
			"type":         "my-type",
			"name":         "my-name",
			"ca-cert-path": "no-such-file",
		},
		err: `open .*\.juju/no-such-file: .*`,
	}, {
		about: "CA key specified as non-existent file",
		attrs: attrs{
			"type":                "my-type",
			"name":                "my-name",
			"ca-private-key-path": "no-such-file",
		},
		err: `open .*\.juju/no-such-file: .*`,
	}, {
		about: "Specified agent version",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"agent-version":   "1.2.3",
		},
	}, {
		about: "Specified development flag",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"development":     true,
		},
	}, {
		about: "Specified admin secret",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"development":     false,
			"admin-secret":    "pork",
		},
	}, {
		about: "Invalid development flag",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"development":     "true",
		},
		err: "development: expected bool, got \"true\"",
	}, {
		about: "Invalid agent version",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"agent-version":   "2",
		},
		err: `invalid agent version in environment configuration: "2"`,
	}, {
		about: "Missing type",
		attrs: attrs{
			"name": "my-name",
		},
		err: "type: expected string, got nothing",
	}, {
		about: "Empty type",
		attrs: attrs{
			"name": "my-name",
			"type": "",
		},
		err: "empty type in environment configuration",
	}, {
		about: "Missing name",
		attrs: attrs{
			"type": "my-type",
		},
		err: "name: expected string, got nothing",
	}, {
		about: "Bad name, no slash",
		attrs: attrs{
			"name": "foo/bar",
			"type": "my-type",
		},
		err: "environment name contains unsafe characters",
	}, {
		about: "Bad name, no backslash",
		attrs: attrs{
			"name": "foo\\bar",
			"type": "my-type",
		},
		err: "environment name contains unsafe characters",
	}, {
		about: "Empty name",
		attrs: attrs{
			"type": "my-type",
			"name": "",
		},
		err: "empty name in environment configuration",
	}, {
		about: "Default firewall mode",
		attrs: attrs{
			"type":          "my-type",
			"name":          "my-name",
			"firewall-mode": config.FwDefault,
		},
	}, {
		about: "Instance firewall mode",
		attrs: attrs{
			"type":          "my-type",
			"name":          "my-name",
			"firewall-mode": config.FwInstance,
		},
	}, {
		about: "Global firewall mode",
		attrs: attrs{
			"type":          "my-type",
			"name":          "my-name",
			"firewall-mode": config.FwGlobal,
		},
	}, {
		about: "Illegal firewall mode",
		attrs: attrs{
			"type":          "my-type",
			"name":          "my-name",
			"firewall-mode": "illegal",
		},
		err: "invalid firewall mode in environment configuration: .*",
	}, {
		about: "ssl-hostname-verification off",
		attrs: attrs{
			"type": "my-type",
			"name": "my-name",
			"ssl-hostname-verification": false,
		},
	}, {
		about: "ssl-hostname-verification incorrect",
		attrs: attrs{
			"type": "my-type",
			"name": "my-name",
			"ssl-hostname-verification": "yes please",
		},
		err: `ssl-hostname-verification: expected bool, got "yes please"`,
	}, {
		about: "Explicit state port",
		attrs: attrs{
			"type":       "my-type",
			"name":       "my-name",
			"state-port": 37042,
		},
	}, {
		about: "Invalid state port",
		attrs: attrs{
			"type":       "my-type",
			"name":       "my-name",
			"state-port": "illegal",
		},
		err: `state-port: expected number, got "illegal"`,
	}, {
		about: "Explicit API port",
		attrs: attrs{
			"type":     "my-type",
			"name":     "my-name",
			"api-port": 77042,
		},
	}, {
		about: "Invalid API port",
		attrs: attrs{
			"type":     "my-type",
			"name":     "my-name",
			"api-port": "illegal",
		},
		err: `api-port: expected number, got "illegal"`,
	},
}

type testFile struct {
	name, data string
}

func (*ConfigSuite) TestConfig(c *gc.C) {
	files := []testing.TestFile{
		{".ssh/id_dsa.pub", "dsa"},
		{".ssh/id_rsa.pub", "rsa\n"},
		{".ssh/identity.pub", "identity"},
		{".ssh/authorized_keys", "auth0\n# first\nauth1\n\n"},
		{".ssh/authorized_keys2", "auth2\nauth3\n"},

		{".juju/my-name-cert.pem", caCert},
		{".juju/my-name-private-key.pem", caKey},
		{".juju/cacert2.pem", caCert2},
		{".juju/cakey2.pem", caKey2},
		{"othercert.pem", caCert3},
		{"otherkey.pem", caKey3},
	}
	h := testing.MakeFakeHomeWithFiles(c, files)
	defer h.Restore()
	for i, test := range configTests {
		c.Logf("test %d. %s", i, test.about)
		test.check(c, h)
	}
}

var noCertFilesTests = []configTest{
	{
		about: "Unspecified certificate and key",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
		},
	}, {
		about: "Unspecified certificate, specified key",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"ca-private-key":  caKey,
		},
		err: "bad CA certificate/key in configuration: crypto/tls:.*",
	},
}

func (*ConfigSuite) TestConfigNoCertFiles(c *gc.C) {
	h := testing.MakeEmptyFakeHome(c)
	defer h.Restore()
	for i, test := range noCertFilesTests {
		c.Logf("test %d. %s", i, test.about)
		test.check(c, h)
	}
}

var emptyCertFilesTests = []configTest{
	{
		about: "Cert unspecified; key specified",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"ca-private-key":  caKey,
		},
		err: `bad CA certificate/key in configuration: crypto/tls: .*`,
	}, {
		about: "Cert and key unspecified",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
		},
		err: `bad CA certificate/key in configuration: crypto/tls: .*`,
	}, {
		about: "Cert specified, key unspecified",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"ca-cert":         caCert,
		},
		err: "bad CA certificate/key in configuration: crypto/tls: .*",
	}, {
		about: "Cert and key specified as absent",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"ca-cert":         "",
			"ca-private-key":  "",
		},
	}, {
		about: "Cert specified as absent",
		attrs: attrs{
			"type":            "my-type",
			"name":            "my-name",
			"authorized-keys": "my-keys",
			"ca-cert":         "",
		},
		err: "bad CA certificate/key in configuration: crypto/tls: .*",
	},
}

func (*ConfigSuite) TestConfigEmptyCertFiles(c *gc.C) {
	files := []testing.TestFile{
		{".juju/my-name-cert.pem", ""},
		{".juju/my-name-private-key.pem", ""},
	}
	h := testing.MakeFakeHomeWithFiles(c, files)
	defer h.Restore()

	for i, test := range emptyCertFilesTests {
		c.Logf("test %d. %s", i, test.about)
		test.check(c, h)
	}
}

func (test configTest) check(c *gc.C, home *testing.FakeHome) {
	cfg, err := config.New(test.attrs)
	if test.err != "" {
		c.Check(cfg, gc.IsNil)
		c.Assert(err, gc.ErrorMatches, test.err)
		return
	}
	c.Assert(err, gc.IsNil)

	typ, _ := test.attrs["type"].(string)
	name, _ := test.attrs["name"].(string)
	c.Assert(cfg.Type(), gc.Equals, typ)
	c.Assert(cfg.Name(), gc.Equals, name)
	agentVersion, ok := cfg.AgentVersion()
	if s := test.attrs["agent-version"]; s != nil {
		c.Assert(ok, jc.IsTrue)
		c.Assert(agentVersion, gc.Equals, version.MustParse(s.(string)))
	} else {
		c.Assert(ok, jc.IsFalse)
		c.Assert(agentVersion, gc.Equals, version.Zero)
	}

	if statePort, _ := test.attrs["state-port"].(int); statePort != 0 {
		c.Assert(cfg.StatePort(), gc.Equals, statePort)
	}
	if apiPort, _ := test.attrs["api-port"].(int); apiPort != 0 {
		c.Assert(cfg.APIPort(), gc.Equals, apiPort)
	}

	dev, _ := test.attrs["development"].(bool)
	c.Assert(cfg.Development(), gc.Equals, dev)

	if series, _ := test.attrs["default-series"].(string); series != "" {
		c.Assert(cfg.DefaultSeries(), gc.Equals, series)
	} else {
		c.Assert(cfg.DefaultSeries(), gc.Equals, config.DefaultSeries)
	}

	if m, _ := test.attrs["firewall-mode"].(string); m != "" {
		c.Assert(cfg.FirewallMode(), gc.Equals, config.FirewallMode(m))
	}

	if secret, _ := test.attrs["admin-secret"].(string); secret != "" {
		c.Assert(cfg.AdminSecret(), gc.Equals, secret)
	}

	if path, _ := test.attrs["authorized-keys-path"].(string); path != "" {
		c.Assert(cfg.AuthorizedKeys(), gc.Equals, home.FileContents(c, path))
		c.Assert(cfg.AllAttrs()["authorized-keys-path"], gc.IsNil)
	} else if keys, _ := test.attrs["authorized-keys"].(string); keys != "" {
		c.Assert(cfg.AuthorizedKeys(), gc.Equals, keys)
	} else {
		// Content of all the files that are read by default.
		want := "dsa\nrsa\nidentity\n"
		c.Assert(cfg.AuthorizedKeys(), gc.Equals, want)
	}

	cert, certPresent := cfg.CACert()
	if path, _ := test.attrs["ca-cert-path"].(string); path != "" {
		c.Assert(certPresent, jc.IsTrue)
		c.Assert(string(cert), gc.Equals, home.FileContents(c, path))
	} else if v, ok := test.attrs["ca-cert"].(string); v != "" {
		c.Assert(certPresent, jc.IsTrue)
		c.Assert(string(cert), gc.Equals, v)
	} else if ok {
		c.Check(cert, gc.HasLen, 0)
		c.Assert(certPresent, jc.IsFalse)
	} else if home.FileExists(".juju/my-name-cert.pem") {
		c.Assert(certPresent, jc.IsTrue)
		c.Assert(string(cert), gc.Equals, home.FileContents(c, "my-name-cert.pem"))
	} else {
		c.Check(cert, gc.HasLen, 0)
		c.Assert(certPresent, jc.IsFalse)
	}

	key, keyPresent := cfg.CAPrivateKey()
	if path, _ := test.attrs["ca-private-key-path"].(string); path != "" {
		c.Assert(keyPresent, jc.IsTrue)
		c.Assert(string(key), gc.Equals, home.FileContents(c, path))
	} else if v, ok := test.attrs["ca-private-key"].(string); v != "" {
		c.Assert(keyPresent, jc.IsTrue)
		c.Assert(string(key), gc.Equals, v)
	} else if ok {
		c.Check(key, gc.HasLen, 0)
		c.Assert(keyPresent, jc.IsFalse)
	} else if home.FileExists(".juju/my-name-private-key.pem") {
		c.Assert(keyPresent, jc.IsTrue)
		c.Assert(string(key), gc.Equals, home.FileContents(c, "my-name-private-key.pem"))
	} else {
		c.Check(key, gc.HasLen, 0)
		c.Assert(keyPresent, jc.IsFalse)
	}

	if v, ok := test.attrs["ssl-hostname-verification"]; ok {
		c.Assert(cfg.SSLHostnameVerification(), gc.Equals, v)
	}

	url, urlPresent := cfg.ImageMetadataURL()
	if v, ok := test.attrs["image-metadata-url"]; ok {
		c.Assert(url, gc.Equals, v)
		c.Assert(urlPresent, jc.IsTrue)
	} else {
		c.Assert(urlPresent, jc.IsFalse)
	}
	url, urlPresent = cfg.ToolsURL()
	if v, ok := test.attrs["tools-url"]; ok {
		c.Assert(url, gc.Equals, v)
		c.Assert(urlPresent, jc.IsTrue)
	} else {
		c.Assert(urlPresent, jc.IsFalse)
	}
}

func (*ConfigSuite) TestConfigAttrs(c *gc.C) {
	attrs := map[string]interface{}{
		"type":                      "my-type",
		"name":                      "my-name",
		"authorized-keys":           "my-keys",
		"firewall-mode":             string(config.FwDefault),
		"admin-secret":              "foo",
		"unknown":                   "my-unknown",
		"ca-private-key":            "",
		"ca-cert":                   caCert,
		"ssl-hostname-verification": true,
		"image-metadata-url":        "",
		"tools-url":                 "",
	}
	cfg, err := config.New(attrs)
	c.Assert(err, gc.IsNil)

	// These attributes are added if not set.
	attrs["development"] = false
	attrs["default-series"] = config.DefaultSeries
	// Default firewall mode is instance
	attrs["firewall-mode"] = string(config.FwInstance)
	c.Assert(cfg.AllAttrs(), gc.DeepEquals, attrs)
	c.Assert(cfg.UnknownAttrs(), gc.DeepEquals, map[string]interface{}{"unknown": "my-unknown"})

	newcfg, err := cfg.Apply(map[string]interface{}{
		"name":        "new-name",
		"new-unknown": "my-new-unknown",
	})

	attrs["name"] = "new-name"
	attrs["new-unknown"] = "my-new-unknown"
	c.Assert(newcfg.AllAttrs(), gc.DeepEquals, attrs)
}

type validationTest struct {
	about string
	new   attrs
	old   attrs
	err   string
}

var validationTests = []validationTest{{
	about: "Can't change the type",
	new:   attrs{"type": "new-type"},
	err:   `cannot change type from "my-type" to "new-type"`,
}, {
	about: "Can't change the name",
	new:   attrs{"name": "new-name"},
	err:   `cannot change name from "my-name" to "new-name"`,
}, {
	about: "Can set agent version",
	new:   attrs{"agent-version": "1.9.13"},
}, {
	about: "Can change agent version",
	old:   attrs{"agent-version": "1.9.13"},
	new:   attrs{"agent-version": "1.9.27"},
}, {
	about: "Can't clear agent version",
	old:   attrs{"agent-version": "1.9.27"},
	err:   `cannot clear agent-version`,
}, {
	about: "Can't change the firewall-mode",
	old:   attrs{"firewall-mode": config.FwGlobal},
	new:   attrs{"firewall-mode": config.FwInstance},
	err:   `cannot change firewall-mode from "global" to "instance"`,
}, {
	about: "Cannot change the state-port",
	old:   attrs{"state-port": config.DefaultStatePort},
	new:   attrs{"state-port": 42},
	err:   `cannot change state-port from 37017 to 42`,
}, {
	about: "Cannot change the api-port",
	old:   attrs{"api-port": config.DefaultApiPort},
	new:   attrs{"api-port": 42},
	err:   `cannot change api-port from 17070 to 42`,
}, {
	about: "Can change the state-port from explicit-default to implicit-default",
	old:   attrs{"state-port": config.DefaultStatePort},
}, {
	about: "Can change the api-port from explicit-default to implicit-default",
	old:   attrs{"api-port": config.DefaultApiPort},
}, {
	about: "Can change the state-port from implicit-default to explicit-default",
	new:   attrs{"state-port": config.DefaultStatePort},
}, {
	about: "Can change the api-port from implicit-default to explicit-default",
	new:   attrs{"api-port": config.DefaultApiPort},
}, {
	about: "Cannot change the state-port from implicit-default to different value",
	new:   attrs{"state-port": 42},
	err:   `cannot change state-port from 37017 to 42`,
}, {
	about: "Cannot change the api-port from implicit-default to different value",
	new:   attrs{"api-port": 42},
	err:   `cannot change api-port from 17070 to 42`,
}}

func (*ConfigSuite) TestValidateChange(c *gc.C) {
	files := []testing.TestFile{
		{".ssh/identity.pub", "identity"},
	}
	h := testing.MakeFakeHomeWithFiles(c, files)
	defer h.Restore()

	for i, test := range validationTests {
		c.Logf("test %d: %s", i, test.about)
		newConfig := newTestConfig(c, test.new)
		oldConfig := newTestConfig(c, test.old)
		err := config.Validate(newConfig, oldConfig)
		if test.err == "" {
			c.Assert(err, gc.IsNil)
		} else {
			c.Assert(err, gc.ErrorMatches, test.err)
		}
	}
}

func (*ConfigSuite) TestValidateUnknownAttrs(c *gc.C) {
	defer testing.MakeFakeHomeWithFiles(c, []testing.TestFile{
		{".ssh/id_rsa.pub", "rsa\n"},
		{".juju/myenv-cert.pem", caCert},
		{".juju/myenv-private-key.pem", caKey},
	}).Restore()
	cfg, err := config.New(map[string]interface{}{
		"name":    "myenv",
		"type":    "other",
		"known":   "this",
		"unknown": "that",
	})

	// No fields: all attrs passed through.
	attrs, err := cfg.ValidateUnknownAttrs(nil, nil)
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, gc.DeepEquals, map[string]interface{}{
		"known":   "this",
		"unknown": "that",
	})

	// Valid field: that and other attrs passed through.
	fields := schema.Fields{"known": schema.String()}
	attrs, err = cfg.ValidateUnknownAttrs(fields, nil)
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, gc.DeepEquals, map[string]interface{}{
		"known":   "this",
		"unknown": "that",
	})

	// Default field: inserted.
	fields["default"] = schema.String()
	defaults := schema.Defaults{"default": "the other"}
	attrs, err = cfg.ValidateUnknownAttrs(fields, defaults)
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, gc.DeepEquals, map[string]interface{}{
		"known":   "this",
		"unknown": "that",
		"default": "the other",
	})

	// Invalid field: failure.
	fields["known"] = schema.Int()
	_, err = cfg.ValidateUnknownAttrs(fields, defaults)
	c.Assert(err, gc.ErrorMatches, `known: expected int, got "this"`)
}

func newTestConfig(c *gc.C, explicit attrs) *config.Config {
	final := attrs{"type": "my-type", "name": "my-name"}
	for key, value := range explicit {
		final[key] = value
	}
	result, err := config.New(final)
	c.Assert(err, gc.IsNil)
	return result
}

func (*ConfigSuite) TestGenerateStateServerCertAndKey(c *gc.C) {
	// In order to test missing certs, it checks the JUJU_HOME dir, so we need
	// a fake home.
	defer testing.MakeFakeHomeWithFiles(c, []testing.TestFile{
		{".ssh/id_rsa.pub", "rsa\n"},
	}).Restore()

	for _, test := range []struct {
		configValues map[string]interface{}
		errMatch     string
	}{{
		configValues: map[string]interface{}{
			"name": "test-no-certs",
			"type": "dummy",
		},
		errMatch: "environment configuration has no ca-cert",
	}, {
		configValues: map[string]interface{}{
			"name":    "test-no-certs",
			"type":    "dummy",
			"ca-cert": testing.CACert,
		},
		errMatch: "environment configuration has no ca-private-key",
	}, {
		configValues: map[string]interface{}{
			"name":           "test-no-certs",
			"type":           "dummy",
			"ca-cert":        testing.CACert,
			"ca-private-key": testing.CAKey,
		},
	}} {
		cfg, err := config.New(test.configValues)
		c.Assert(err, gc.IsNil)
		certPEM, keyPEM, err := cfg.GenerateStateServerCertAndKey()
		if test.errMatch == "" {
			c.Assert(err, gc.IsNil)

			_, _, err = cert.ParseCertAndKey(certPEM, keyPEM)
			c.Check(err, gc.IsNil)

			err = cert.Verify(certPEM, []byte(testing.CACert), time.Now())
			c.Assert(err, gc.IsNil)
			err = cert.Verify(certPEM, []byte(testing.CACert), time.Now().AddDate(9, 0, 0))
			c.Assert(err, gc.IsNil)
			err = cert.Verify(certPEM, []byte(testing.CACert), time.Now().AddDate(10, 0, 1))
			c.Assert(err, gc.NotNil)
		} else {
			c.Assert(err, gc.ErrorMatches, test.errMatch)
			c.Assert(certPEM, gc.IsNil)
			c.Assert(keyPEM, gc.IsNil)
		}
	}
}

var caCert = `
-----BEGIN CERTIFICATE-----
MIIBjDCCATigAwIBAgIBADALBgkqhkiG9w0BAQUwHjENMAsGA1UEChMEanVqdTEN
MAsGA1UEAxMEcm9vdDAeFw0xMjExMDkxNjQwMjhaFw0yMjExMDkxNjQ1MjhaMB4x
DTALBgNVBAoTBGp1anUxDTALBgNVBAMTBHJvb3QwWTALBgkqhkiG9w0BAQEDSgAw
RwJAduA1Gnb2VJLxNGfG4St0Qy48Y3q5Z5HheGtTGmti/FjlvQvScCFGCnJG7fKA
Knd7ia3vWg7lxYkIvMPVP88LAQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAKQwEgYD
VR0TAQH/BAgwBgEB/wIBATAdBgNVHQ4EFgQUlvKX8vwp0o+VdhdhoA9O6KlOm00w
HwYDVR0jBBgwFoAUlvKX8vwp0o+VdhdhoA9O6KlOm00wCwYJKoZIhvcNAQEFA0EA
LlNpevtFr8gngjAFFAO/FXc7KiZcCrA5rBfb/rEy297lIqmKt5++aVbLEPyxCIFC
r71Sj63TUTFWtRZAxvn9qQ==
-----END CERTIFICATE-----
`[1:]

var caKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJAduA1Gnb2VJLxNGfG4St0Qy48Y3q5Z5HheGtTGmti/FjlvQvScCFG
CnJG7fKAKnd7ia3vWg7lxYkIvMPVP88LAQIDAQABAkEAsFOdMSYn+AcF1M/iBfjo
uQWJ+Zz+CgwuvumjGNsUtmwxjA+hh0fCn0Ah2nAt4Ma81vKOKOdQ8W6bapvsVDH0
6QIhAJOkLmEKm4H5POQV7qunRbRsLbft/n/SHlOBz165WFvPAiEAzh9fMf70std1
sVCHJRQWKK+vw3oaEvPKvkPiV5ui0C8CIGNsvybuo8ald5IKCw5huRlFeIxSo36k
m3OVCXc6zfwVAiBnTUe7WcivPNZqOC6TAZ8dYvdWo4Ifz3jjpEfymjid1wIgBIJv
ERPyv2NQqIFQZIyzUP7LVRIWfpFFOo9/Ww/7s5Y=
-----END RSA PRIVATE KEY-----
`[1:]

var caCert2 = `
-----BEGIN CERTIFICATE-----
MIIBjTCCATmgAwIBAgIBADALBgkqhkiG9w0BAQUwHjENMAsGA1UEChMEanVqdTEN
MAsGA1UEAxMEcm9vdDAeFw0xMjExMDkxNjQxMDhaFw0yMjExMDkxNjQ2MDhaMB4x
DTALBgNVBAoTBGp1anUxDTALBgNVBAMTBHJvb3QwWjALBgkqhkiG9w0BAQEDSwAw
SAJBAJkSWRrr81y8pY4dbNgt+8miSKg4z6glp2KO2NnxxAhyyNtQHKvC+fJALJj+
C2NhuvOv9xImxOl3Hg8fFPCXCtcCAwEAAaNmMGQwDgYDVR0PAQH/BAQDAgCkMBIG
A1UdEwEB/wQIMAYBAf8CAQEwHQYDVR0OBBYEFOsX/ZCqKzWCAaTTVcWsWKT5Msow
MB8GA1UdIwQYMBaAFOsX/ZCqKzWCAaTTVcWsWKT5MsowMAsGCSqGSIb3DQEBBQNB
AAVV57jetEzJQnjgBzhvx/UwauFn78jGhXfV5BrQmxIb4SF4DgSCFstPwUQOAr8h
XXzJqBQH92KYmp+y3YXDoMQ=
-----END CERTIFICATE-----
`[1:]

var caKey2 = `
-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJBAJkSWRrr81y8pY4dbNgt+8miSKg4z6glp2KO2NnxxAhyyNtQHKvC
+fJALJj+C2NhuvOv9xImxOl3Hg8fFPCXCtcCAwEAAQJATQNzO11NQvJS5U6eraFt
FgSFQ8XZjILtVWQDbJv8AjdbEgKMHEy33icsAKIUAx8jL9kjq6K9kTdAKXZi9grF
UQIhAPD7jccIDUVm785E5eR9eisq0+xpgUIa24Jkn8cAlst5AiEAopxVFl1auer3
GP2In3pjdL4ydzU/gcRcYisoJqwHpM8CIHtqmaXBPeq5WT9ukb5/dL3+5SJCtmxA
jQMuvZWRe6khAiBvMztYtPSDKXRbCZ4xeQ+kWSDHtok8Y5zNoTeu4nvDrwIgb3Al
fikzPveC5g6S6OvEQmyDz59tYBubm2XHgvxqww0=
-----END RSA PRIVATE KEY-----
`[1:]

var caCert3 = `
-----BEGIN CERTIFICATE-----
MIIBjTCCATmgAwIBAgIBADALBgkqhkiG9w0BAQUwHjENMAsGA1UEChMEanVqdTEN
MAsGA1UEAxMEcm9vdDAeFw0xMjExMDkxNjQxMjlaFw0yMjExMDkxNjQ2MjlaMB4x
DTALBgNVBAoTBGp1anUxDTALBgNVBAMTBHJvb3QwWjALBgkqhkiG9w0BAQEDSwAw
SAJBAIW7CbHFJivvV9V6mO8AGzJS9lqjUf6MdEPsdF6wx2Cpzr/lSFIggCwRA138
9MuFxflxb/3U8Nq+rd8rVtTgFMECAwEAAaNmMGQwDgYDVR0PAQH/BAQDAgCkMBIG
A1UdEwEB/wQIMAYBAf8CAQEwHQYDVR0OBBYEFJafrxqByMN9BwGfcmuF0Lw/1QII
MB8GA1UdIwQYMBaAFJafrxqByMN9BwGfcmuF0Lw/1QIIMAsGCSqGSIb3DQEBBQNB
AHq3vqNhxya3s33DlQfSj9whsnqM0Nm+u8mBX/T76TF5rV7+B33XmYzSyfA3yBi/
zHaUR/dbHuiNTO+KXs3/+Y4=
-----END CERTIFICATE-----
`[1:]

var caKey3 = `
-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBAIW7CbHFJivvV9V6mO8AGzJS9lqjUf6MdEPsdF6wx2Cpzr/lSFIg
gCwRA1389MuFxflxb/3U8Nq+rd8rVtTgFMECAwEAAQJAaivPi4qJPrJb2onl50H/
VZnWKqmljGF4YQDWduMEt7GTPk+76x9SpO7W4gfY490Ivd9DEXfbr/KZqhwWikNw
LQIhALlLfRXLF2ZfToMfB1v1v+jith5onAu24O68mkdRc5PLAiEAuMJ/6U07hggr
Ckf9OT93wh84DK66h780HJ/FUHKcoCMCIDsPZaJBpoa50BOZG0ZjcTTwti3BGCPf
uZg+w0oCGz27AiEAsUCYKqEXy/ymHhT2kSecozYENdajyXvcaOG3EPkD3nUCICOP
zatzs7c/4mx4a0JBG6Za0oEPUcm2I34is50KSohz
-----END RSA PRIVATE KEY-----
`[1:]

var invalidCAKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJAZabKgKInuOxj5vDWLwHHQtK3/45KB+32D15w94Nt83BmuGxo90lw
-----END RSA PRIVATE KEY-----
`[1:]

var invalidCACert = `
-----BEGIN CERTIFICATE-----
MIIBOgIBAAJAZabKgKInuOxj5vDWLwHHQtK3/45KB+32D15w94Nt83BmuGxo90lw
-----END CERTIFICATE-----
`[1:]
