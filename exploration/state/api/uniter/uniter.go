// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter

import (
	"fmt"

	"launchpad.net/juju-core/state/api/common"
	"launchpad.net/juju-core/state/api/params"
)

// State provides access to the Uniter API facade.
type State struct {
	caller common.Caller
}

// NewState creates a new client-side Uniter facade.
func NewState(caller common.Caller) *State {
	return &State{caller}
}

// unitLife requests the lifecycle of the given unit from the server.
func (st *State) unitLife(tag string) (params.Life, error) {
	var result params.LifeResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: tag}},
	}
	err := st.caller.Call("Uniter", "", "Life", args, &result)
	if err != nil {
		return "", err
	}
	if len(result.Results) != 1 {
		return "", fmt.Errorf("expected one result, got %d", len(result.Results))
	}
	if err := result.Results[0].Error; err != nil {
		return "", err
	}
	return result.Results[0].Life, nil
}

// Unit provides access to methods of a state.Unit through the facade.
func (st *State) Unit(tag string) (*Unit, error) {
	life, err := st.unitLife(tag)
	if err != nil {
		return nil, err
	}
	return &Unit{
		tag:  tag,
		life: life,
		st:   st,
	}, nil
}
