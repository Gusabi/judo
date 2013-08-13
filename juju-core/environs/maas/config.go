// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package maas

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/schema"
)

var configFields = schema.Fields{
	"maas-server": schema.String(),
	// maas-oauth is a colon-separated triplet of:
	// consumer-key:resource-token:resource-secret
	"maas-oauth": schema.String(),
}
var configDefaults = schema.Defaults{}

type maasEnvironConfig struct {
	*config.Config
	attrs map[string]interface{}
}

func (cfg *maasEnvironConfig) MAASServer() string {
	return cfg.attrs["maas-server"].(string)
}

func (cfg *maasEnvironConfig) MAASOAuth() string {
	return cfg.attrs["maas-oauth"].(string)
}

func (prov maasEnvironProvider) newConfig(cfg *config.Config) (*maasEnvironConfig, error) {
	validCfg, err := prov.Validate(cfg, nil)
	if err != nil {
		return nil, err
	}
	result := new(maasEnvironConfig)
	result.Config = validCfg
	result.attrs = validCfg.UnknownAttrs()
	return result, nil
}

var errMalformedMaasOAuth = errors.New("malformed maas-oauth (3 items separated by colons)")

func (prov maasEnvironProvider) Validate(cfg, oldCfg *config.Config) (*config.Config, error) {
	// Validate base configuration change before validating MAAS specifics.
	err := config.Validate(cfg, oldCfg)
	if err != nil {
		return nil, err
	}

	validated, err := cfg.ValidateUnknownAttrs(configFields, configDefaults)
	if err != nil {
		return nil, err
	}
	envCfg := new(maasEnvironConfig)
	envCfg.Config = cfg
	envCfg.attrs = validated
	server := envCfg.MAASServer()
	serverURL, err := url.Parse(server)
	if err != nil || serverURL.Scheme == "" || serverURL.Host == "" {
		return nil, fmt.Errorf("malformed maas-server URL '%v': %s", server, err)
	}
	oauth := envCfg.MAASOAuth()
	if strings.Count(oauth, ":") != 2 {
		return nil, errMalformedMaasOAuth
	}
	return cfg.Apply(envCfg.attrs)
}
