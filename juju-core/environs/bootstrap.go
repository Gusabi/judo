// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package environs

import (
	"fmt"
	"time"

	"launchpad.net/juju-core/agent"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/errors"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/utils"
	"launchpad.net/juju-core/version"
)

// Use ShortAttempt to poll for short-term events.
// TODO: This may need tuning for different providers (or even environments).
var ShortAttempt = utils.AttemptStrategy{
	Total: 5 * time.Second,
	Delay: 200 * time.Millisecond,
}

// Bootstrap bootstraps the given environment. The supplied constraints are
// used to provision the instance, and are also set within the bootstrapped
// environment.
func Bootstrap(environ Environ, cons constraints.Value) error {
	cfg := environ.Config()
	if secret := cfg.AdminSecret(); secret == "" {
		return fmt.Errorf("environment configuration has no admin-secret")
	}
	if authKeys := cfg.AuthorizedKeys(); authKeys == "" {
		// Apparently this can never happen, so it's not tested. But, one day,
		// Config will act differently (it's pretty crazy that, AFAICT, the
		// authorized-keys are optional config settings... but it's impossible
		// to actually *create* a config without them)... and when it does,
		// we'll be here to catch this problem early.
		return fmt.Errorf("environment configuration has no authorized-keys")
	}
	if _, hasCACert := cfg.CACert(); !hasCACert {
		return fmt.Errorf("environment configuration has no ca-cert")
	}
	if _, hasCAKey := cfg.CAPrivateKey(); !hasCAKey {
		return fmt.Errorf("environment configuration has no ca-private-key")
	}
	// If the state file exists, it might actually have just been
	// removed by Destroy, and eventual consistency has not caught
	// up yet, so we retry to verify if that is happening.
	err := verifyBootstrapInit(environ)
	if err != nil {
		return err
	}
	return environ.Bootstrap(cons)
}

// verifyBootstrapInit does the common initial check before bootstrapping, to
// confirm that the environment isn't already running, and that the storage
// works.
func verifyBootstrapInit(env Environ) error {
	var err error

	storage := env.Storage()

	// If the state file exists, it might actually have just been
	// removed by Destroy, and eventual consistency has not caught
	// up yet, so we retry to verify if that is happening.
	for a := storage.ConsistencyStrategy().Start(); a.Next(); {
		if _, err = LoadState(storage); err != nil {
			break
		}
	}
	if err == nil {
		return fmt.Errorf("environment is already bootstrapped")
	}
	if !errors.IsNotFoundError(err) {
		return fmt.Errorf("cannot query old bootstrap state: %v", err)
	}

	return VerifyStorage(storage)
}

// BootstrapUsers creates the initial admin user for the database, and sets
// the initial password.
func BootstrapUsers(st *state.State, cfg *config.Config, passwordHash string) error {
	logger.Debugf("adding admin user")
	// Set up initial authentication.
	u, err := st.AddUser("admin", "")
	if err != nil {
		return err
	}

	// Note that at bootstrap time, the password is set to
	// the hash of its actual value. The first time a client
	// connects to mongo, it changes the mongo password
	// to the original password.
	logger.Debugf("setting password hash for admin user")
	if err := u.SetPasswordHash(passwordHash); err != nil {
		return err
	}
	if err := st.SetAdminMongoPassword(passwordHash); err != nil {
		return err
	}
	return nil

}

// ConfigureBootstrapMachine adds the initial machine into state.  As a part
// of this process the environmental constraints are saved as constraints used
// when bootstrapping are considered constraints for the entire environment.
func ConfigureBootstrapMachine(
	st *state.State,
	cons constraints.Value,
	datadir string,
	jobs []state.MachineJob,
	instId instance.Id,
	characteristics instance.HardwareCharacteristics,
) error {
	logger.Debugf("setting environment constraints")
	if err := st.SetEnvironConstraints(cons); err != nil {
		return err
	}

	logger.Debugf("create bootstrap machine in state")
	m, err := st.InjectMachine(version.Current.Series, cons, instId, characteristics, jobs...)
	if err != nil {
		return err
	}
	// Read the machine agent's password and change it to
	// a new password (other agents will change their password
	// via the API connection).
	logger.Debugf("create new random password for machine %v", m.Id())
	mconf, err := agent.ReadConf(datadir, m.Tag())
	if err != nil {
		return err
	}
	newPassword, err := utils.RandomPassword()
	if err != nil {
		return err
	}
	mconf.StateInfo.Password = newPassword
	mconf.APIInfo.Password = newPassword
	mconf.OldPassword = ""

	if err := mconf.Write(); err != nil {
		return err
	}
	if err := m.SetMongoPassword(newPassword); err != nil {
		return err
	}
	if err := m.SetPassword(newPassword); err != nil {
		return err
	}
	return nil
}
