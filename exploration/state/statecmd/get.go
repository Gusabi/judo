// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// The statecmd package is a temporary package
// to put code that's used by both cmd/juju and state/api.
// It is intended to wither away to nothing as functionality
// gets absorbed into state and state/api as appropriate
// when the command-line commands can invoke the
// API directly.
package statecmd

import (
	"launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
)

// ServiceGet returns the configuration for the named service.
func ServiceGet(st *state.State, p params.ServiceGet) (params.ServiceGetResults, error) {
	service, err := st.Service(p.ServiceName)
	if err != nil {
		return params.ServiceGetResults{}, err
	}
	settings, err := service.ConfigSettings()
	if err != nil {
		return params.ServiceGetResults{}, err
	}
	charm, _, err := service.Charm()
	if err != nil {
		return params.ServiceGetResults{}, err
	}
	configInfo := describe(settings, charm.Config())
	var constraints constraints.Value
	if service.IsPrincipal() {
		constraints, err = service.Constraints()
		if err != nil {
			return params.ServiceGetResults{}, err
		}
	}
	return params.ServiceGetResults{
		Service:     p.ServiceName,
		Charm:       charm.Meta().Name,
		Config:      configInfo,
		Constraints: constraints,
	}, nil
}

func describe(settings charm.Settings, config *charm.Config) map[string]interface{} {
	results := make(map[string]interface{})
	for name, option := range config.Options {
		info := map[string]interface{}{
			"description": option.Description,
			"type":        option.Type,
		}
		if value := settings[name]; value != nil {
			info["value"] = value
		} else {
			info["value"] = option.Default
			info["default"] = true
		}
		results[name] = info
	}
	return results
}
