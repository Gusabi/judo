// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"io/ioutil"

	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/agent/tools"
	"launchpad.net/juju-core/environs"
	envtesting "launchpad.net/juju-core/environs/testing"
	"launchpad.net/juju-core/juju/testing"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/version"
)

type UpgradeJujuSuite struct {
	testing.JujuConnSuite
}

var _ = Suite(&UpgradeJujuSuite{})

var upgradeJujuTests = []struct {
	about          string
	private        []string
	public         []string
	currentVersion string
	agentVersion   string
	development    bool

	args           []string
	expectInitErr  string
	expectErr      string
	expectVersion  string
	expectUploaded []string
}{{
	about:          "unwanted extra argument",
	currentVersion: "1.0.0-foo-bar",
	args:           []string{"foo"},
	expectInitErr:  "unrecognized args:.*",
}, {
	about:          "invalid --version value",
	currentVersion: "1.0.0-foo-bar",
	args:           []string{"--version", "invalid-version"},
	expectInitErr:  "invalid version .*",
}, {
	about:          "major version upgrade to incompatible version",
	currentVersion: "2.0.0-foo-bar",
	args:           []string{"--version", "5.2.0"},
	expectInitErr:  "cannot upgrade to version incompatible with CLI",
}, {
	about:          "major version downgrade to incompatible version",
	currentVersion: "4.2.0-foo-bar",
	args:           []string{"--version", "3.2.0"},
	expectInitErr:  "cannot upgrade to version incompatible with CLI",
}, {
	about:          "invalid --series",
	currentVersion: "4.2.0-foo-bar",
	args:           []string{"--series", "precise&quantal"},
	expectInitErr:  `invalid value "precise&quantal" for flag --series: .*`,
}, {
	about:          "--series without --upload-tools",
	currentVersion: "4.2.0-foo-bar",
	args:           []string{"--series", "precise,quantal"},
	expectInitErr:  "--series requires --upload-tools",
}, {
	about:          "--upload-tools with inappropriate version 1",
	currentVersion: "4.2.0-foo-bar",
	args:           []string{"--upload-tools", "--version", "3.1.0"},
	expectInitErr:  "cannot upgrade to version incompatible with CLI",
}, {
	about:          "--upload-tools with inappropriate version 2",
	currentVersion: "3.2.7-foo-bar",
	args:           []string{"--upload-tools", "--version", "3.1.0.4"},
	expectInitErr:  "cannot specify build number when uploading tools",
}, {
	about:          "latest release from private storage",
	private:        []string{"2.0.0-foo-bar", "2.0.2-foo-bletch", "2.0.3-foo-bar"},
	public:         []string{"2.0.0-foo-bar", "2.0.4-foo-bar", "2.0.5-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.0.0",
	expectVersion:  "2.0.3",
}, {
	about:          "latest dev from private storage (because client is dev)",
	private:        []string{"2.0.0-foo-bar", "2.2.0-foo-bar", "2.3.0-foo-bar", "3.0.1-foo-bar"},
	public:         []string{"2.0.0-foo-bar", "2.4.0-foo-bar", "2.5.0-foo-bar"},
	currentVersion: "2.1.0-foo-bar",
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from private storage (because agent is dev)",
	private:        []string{"2.0.0-foo-bar", "2.2.0-foo-bar", "2.3.0-foo-bar", "3.0.1-foo-bar"},
	public:         []string{"2.0.0-foo-bar", "2.4.0-foo-bar", "2.5.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.1.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from private storage (because --dev flag)",
	private:        []string{"2.0.0-foo-bar", "2.2.0-foo-bar", "2.3.0-foo-bar"},
	public:         []string{"2.0.0-foo-bar", "2.4.0-foo-bar", "2.5.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	args:           []string{"--dev"},
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from private storage (because dev env setting)",
	private:        []string{"2.0.0-foo-bar", "2.2.0-foo-bar", "2.3.0-foo-bar"},
	public:         []string{"2.0.0-foo-bar", "2.4.0-foo-bar", "2.5.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	development:    true,
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest release from public storage",
	public:         []string{"2.0.0-foo-bar", "2.2.0-arble-bletch", "2.3.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.0.0",
	expectVersion:  "2.2.0",
}, {
	about:          "latest dev from public storage (because client is dev)",
	public:         []string{"2.0.0-foo-bar", "2.2.0-arble-bletch", "2.3.0-foo-bar"},
	currentVersion: "2.1.0-foo-bar",
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from public storage (because agent is dev)",
	public:         []string{"2.0.0-foo-bar", "2.2.0-arble-bletch", "2.3.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.1.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from public storage (because --dev flag)",
	public:         []string{"2.0.0-foo-bar", "2.2.0-arble-bletch", "2.3.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	args:           []string{"--dev"},
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "latest dev from public storage (because dev env setting)",
	public:         []string{"2.0.0-foo-bar", "2.2.0-arble-bletch", "2.3.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	development:    true,
	agentVersion:   "2.0.0",
	expectVersion:  "2.3.0",
}, {
	about:          "specified version",
	public:         []string{"2.3.0-foo-bar"},
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.0.0",
	args:           []string{"--version", "2.3.0"},
	expectVersion:  "2.3.0",
}, {
	about:          "specified version missing, but already set",
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.0.0"},
	expectVersion:  "3.0.0",
}, {
	about:          "specified version, no tools",
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "no tools available",
}, {
	about:          "specified version, no matching major version",
	private:        []string{"4.2.0-foo-bar"},
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "no matching tools available",
}, {
	about:          "specified version, no matching minor version",
	private:        []string{"3.4.0-foo-bar"},
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "no matching tools available",
}, {
	about:          "specified version, no matching patch version",
	private:        []string{"3.2.5-foo-bar"},
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "no matching tools available",
}, {
	about:          "specified version, no matching build version",
	private:        []string{"3.2.0.2-foo-bar"},
	currentVersion: "3.0.0-foo-bar",
	agentVersion:   "3.0.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "no matching tools available",
}, {
	about:          "major version downgrade to compatible version",
	private:        []string{"3.2.0-foo-bar"},
	currentVersion: "3.2.0-foo-bar",
	agentVersion:   "4.2.0",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "cannot change major version from 4 to 3",
}, {
	about:          "major version upgrade to compatible version",
	private:        []string{"3.2.0-foo-bar"},
	currentVersion: "3.2.0-foo-bar",
	agentVersion:   "2.8.2",
	args:           []string{"--version", "3.2.0"},
	expectErr:      "major version upgrades are not supported yet",
}, {
	about:          "nothing available 1",
	currentVersion: "2.0.0-foo-bar",
	agentVersion:   "2.0.0",
	expectVersion:  "2.0.0",
}, {
	about:          "nothing available 2",
	currentVersion: "2.0.0-foo-bar",
	public:         []string{"3.2.0-foo-bar"},
	agentVersion:   "2.0.0",
	expectVersion:  "2.0.0",
}, {
	about:          "nothing available 3",
	currentVersion: "2.0.0-foo-bar",
	private:        []string{"3.2.0-foo-bar"},
	public:         []string{"2.4.0-foo-bar"},
	agentVersion:   "2.0.0",
	expectVersion:  "2.0.0",
}, {
	about:          "upload with default series",
	currentVersion: "2.2.0-foo-bar",
	agentVersion:   "2.0.0",
	args:           []string{"--upload-tools"},
	expectVersion:  "2.2.0.1",
	expectUploaded: []string{"2.2.0.1-foo-bar", "2.2.0.1-precise-bar", "2.2.0.1-always-bar"},
}, {
	about:          "upload with explicit version",
	currentVersion: "2.2.0-foo-bar",
	agentVersion:   "2.0.0",
	args:           []string{"--upload-tools", "--version", "2.7.3"},
	expectVersion:  "2.7.3.1",
	expectUploaded: []string{"2.7.3.1-foo-bar", "2.7.3.1-precise-bar", "2.7.3.1-always-bar"},
}, {
	about:          "upload with explicit series",
	currentVersion: "2.2.0-foo-bar",
	agentVersion:   "2.0.0",
	args:           []string{"--upload-tools", "--series", "plonk"},
	expectVersion:  "2.2.0.1",
	expectUploaded: []string{"2.2.0.1-foo-bar", "2.2.0.1-plonk-bar"},
}, {
	about:          "upload dev version, currently on release version",
	currentVersion: "2.1.0-foo-bar",
	agentVersion:   "2.0.0",
	args:           []string{"--upload-tools"},
	expectVersion:  "2.1.0.1",
	expectUploaded: []string{"2.1.0.1-foo-bar", "2.1.0.1-precise-bar", "2.1.0.1-always-bar"},
}, {
	about:          "upload bumps version when necessary",
	private:        []string{"2.4.6-foo-bar", "2.4.8-foo-bar"},
	public:         []string{"2.4.6.4-foo-bar"}, //ignored
	currentVersion: "2.4.6-foo-bar",
	agentVersion:   "2.4.0",
	args:           []string{"--upload-tools"},
	expectVersion:  "2.4.6.1",
	expectUploaded: []string{"2.4.6.1-foo-bar", "2.4.6.1-precise-bar", "2.4.6.1-always-bar"},
}, {
	about:          "upload re-bumps version when necessary",
	private:        []string{"2.4.6-foo-bar", "2.4.6.2-flim-flam", "2.4.8-foo-bar"},
	public:         []string{"2.4.6.10-foo-bar"}, //ignored
	currentVersion: "2.4.6-foo-bar",
	agentVersion:   "2.4.6.2",
	args:           []string{"--upload-tools"},
	expectVersion:  "2.4.6.3",
	expectUploaded: []string{"2.4.6.3-foo-bar", "2.4.6.3-precise-bar", "2.4.6.3-always-bar"},
}, {
	about:          "upload with explicit version bumps when necessary",
	currentVersion: "2.2.0-foo-bar",
	private:        []string{"2.7.3.1-foo-bar"},
	agentVersion:   "2.0.0",
	args:           []string{"--upload-tools", "--version", "2.7.3"},
	expectVersion:  "2.7.3.2",
	expectUploaded: []string{"2.7.3.2-foo-bar", "2.7.3.2-precise-bar", "2.7.3.2-always-bar"},
}}

// mockUploadTools simulates the effect of tools.Upload, but skips the time-
// consuming build from source.
// TODO(fwereade) better factor agent/tools such that build logic is
// exposed and can itself be neatly mocked?
func mockUploadTools(putter tools.URLPutter, forceVersion *version.Number, series ...string) (*tools.Tools, error) {
	storage := putter.(environs.Storage)
	vers := version.Current
	if forceVersion != nil {
		vers.Number = *forceVersion
	}
	t := envtesting.MustUploadFakeToolsVersion(storage, vers)
	for _, series := range series {
		if series != version.Current.Series {
			vers.Series = series
			envtesting.MustUploadFakeToolsVersion(storage, vers)
		}
	}
	return t, nil
}

func (s *UpgradeJujuSuite) TestUpgradeJuju(c *C) {
	oldVersion := version.Current
	uploadTools = mockUploadTools
	defer func() {
		version.Current = oldVersion
		uploadTools = tools.Upload
	}()

	for i, test := range upgradeJujuTests {
		c.Logf("\ntest %d: %s", i, test.about)
		s.Reset(c)

		// Set up apparent CLI version and initialize the command.
		version.Current = version.MustParseBinary(test.currentVersion)
		com := &UpgradeJujuCommand{}
		if err := coretesting.InitCommand(com, test.args); err != nil {
			if test.expectInitErr != "" {
				c.Check(err, ErrorMatches, test.expectInitErr)
			} else {
				c.Check(err, IsNil)
			}
			continue
		}

		// Set up state and environ, and run the command.
		cfg, err := s.State.EnvironConfig()
		c.Assert(err, IsNil)
		cfg, err = cfg.Apply(map[string]interface{}{
			"agent-version": test.agentVersion,
			"development":   test.development,
		})
		c.Assert(err, IsNil)
		err = s.State.SetEnvironConfig(cfg)
		c.Assert(err, IsNil)
		for _, v := range test.private {
			vers := version.MustParseBinary(v)
			envtesting.MustUploadFakeToolsVersion(s.Conn.Environ.Storage(), vers)
		}
		for _, v := range test.public {
			vers := version.MustParseBinary(v)
			storage := s.Conn.Environ.PublicStorage().(environs.Storage)
			envtesting.MustUploadFakeToolsVersion(storage, vers)
		}
		err = com.Run(coretesting.Context(c))
		if test.expectErr != "" {
			c.Check(err, ErrorMatches, test.expectErr)
			continue
		} else if !c.Check(err, IsNil) {
			continue
		}

		// Check expected changes to environ/state.
		cfg, err = s.State.EnvironConfig()
		c.Check(err, IsNil)
		agentVersion, ok := cfg.AgentVersion()
		c.Check(ok, Equals, true)
		c.Check(agentVersion, Equals, version.MustParse(test.expectVersion))
		c.Check(cfg.Development(), Equals, test.development)

		for _, uploaded := range test.expectUploaded {
			vers := version.MustParseBinary(uploaded)
			r, err := s.Conn.Environ.Storage().Get(tools.StorageName(vers))
			if !c.Check(err, IsNil) {
				continue
			}
			data, err := ioutil.ReadAll(r)
			r.Close()
			c.Check(err, IsNil)
			c.Check(string(data), Equals, uploaded)
		}
	}
}

// JujuConnSuite very helpfully uploads some default
// tools to the environment's storage. We don't want
// 'em there; but we do want a consistent default-series
// in the environment state.
func (s *UpgradeJujuSuite) Reset(c *C) {
	s.JujuConnSuite.Reset(c)
	envtesting.RemoveTools(c, s.Conn.Environ.Storage())
	envtesting.RemoveTools(c, s.Conn.Environ.PublicStorage().(environs.Storage))
	cfg, err := s.State.EnvironConfig()
	c.Assert(err, IsNil)
	cfg, err = cfg.Apply(map[string]interface{}{
		"default-series": "always",
		"agent-version":  "1.2.3",
	})
	c.Assert(err, IsNil)
	err = s.State.SetEnvironConfig(cfg)
	c.Assert(err, IsNil)
}

func (s *UpgradeJujuSuite) TestUpgradeJujuWithRealUpload(c *C) {
	s.Reset(c)
	_, err := coretesting.RunCommand(c, &UpgradeJujuCommand{}, []string{"--upload-tools"})
	c.Assert(err, IsNil)
	vers := version.Current
	vers.Build = 1
	name := tools.StorageName(vers)
	r, err := s.Conn.Environ.Storage().Get(name)
	c.Assert(err, IsNil)
	r.Close()
}
