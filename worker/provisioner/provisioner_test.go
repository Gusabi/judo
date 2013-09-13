// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner_test

import (
	"fmt"
	"strings"
	stdtesting "testing"
	"time"

	"labix.org/v2/mgo/bson"
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/errors"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/names"
	"launchpad.net/juju-core/provider/dummy"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/testing/checkers"
	"launchpad.net/juju-core/utils"
	"launchpad.net/juju-core/utils/set"
	"launchpad.net/juju-core/worker"
	"launchpad.net/juju-core/worker/provisioner"
)

func TestPackage(t *stdtesting.T) {
	coretesting.MgoTestPackage(t)
}

type CommonProvisionerSuite struct {
	testing.JujuConnSuite
	op  <-chan dummy.Operation
	cfg *config.Config
	//  // defaultConstraints are used when adding a machine and then later in test assertions.
	defaultConstraints constraints.Value
}

type ProvisionerSuite struct {
	CommonProvisionerSuite
}

var _ = gc.Suite(&ProvisionerSuite{})

var veryShortAttempt = utils.AttemptStrategy{
	Total: 1 * time.Second,
	Delay: 80 * time.Millisecond,
}

var _ worker.Worker = (*provisioner.Provisioner)(nil)

func (s *CommonProvisionerSuite) SetUpSuite(c *gc.C) {
	s.JujuConnSuite.SetUpSuite(c)
	s.defaultConstraints = constraints.MustParse("arch=amd64 mem=4G cpu-cores=1 root-disk=8G")
}

func (s *CommonProvisionerSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	// Create the operations channel with more than enough space
	// for those tests that don't listen on it.
	op := make(chan dummy.Operation, 500)
	dummy.Listen(op)
	s.op = op

	cfg, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	s.cfg = cfg
}

// breakDummyProvider changes the environment config in state in a way
// that causes the given environMethod of the dummy provider to return
// an error, which is also returned as a message to be checked.
func breakDummyProvider(c *gc.C, st *state.State, environMethod string) string {
	oldCfg, err := st.EnvironConfig()
	c.Assert(err, gc.IsNil)
	cfg, err := oldCfg.Apply(map[string]interface{}{"broken": environMethod})
	c.Assert(err, gc.IsNil)
	err = st.SetEnvironConfig(cfg)
	c.Assert(err, gc.IsNil)
	return fmt.Sprintf("dummy.%s is broken", environMethod)
}

// invalidateEnvironment alters the environment configuration
// so the Settings returned from the watcher will not pass
// validation.
func (s *CommonProvisionerSuite) invalidateEnvironment(c *gc.C) error {
	admindb := s.Session.DB("admin")
	err := admindb.Login("admin", testing.AdminSecret)
	if err != nil {
		err = admindb.Login("admin", utils.PasswordHash(testing.AdminSecret))
	}
	c.Assert(err, gc.IsNil)
	settings := s.Session.DB("juju").C("settings")
	return settings.UpdateId("e", bson.D{{"$unset", bson.D{{"type", 1}}}})
}

// fixEnvironment undoes the work of invalidateEnvironment.
func (s *CommonProvisionerSuite) fixEnvironment() error {
	return s.State.SetEnvironConfig(s.cfg)
}

// stopper is stoppable.
type stopper interface {
	Stop() error
}

// stop stops a stopper.
func stop(c *gc.C, s stopper) {
	c.Assert(s.Stop(), gc.IsNil)
}

func (s *CommonProvisionerSuite) checkStartInstance(c *gc.C, m *state.Machine) instance.Instance {
	return s.checkStartInstanceCustom(c, m, "pork", s.defaultConstraints)
}

func (s *CommonProvisionerSuite) checkStartInstanceCustom(c *gc.C, m *state.Machine, secret string, cons constraints.Value) (inst instance.Instance) {
	s.State.StartSync()
	for {
		select {
		case o := <-s.op:
			switch o := o.(type) {
			case dummy.OpStartInstance:
				inst = o.Instance
				s.waitInstanceId(c, m, inst.Id())

				// Check the instance was started with the expected params.
				c.Assert(o.MachineId, gc.Equals, m.Id())
				nonceParts := strings.SplitN(o.MachineNonce, ":", 2)
				c.Assert(nonceParts, gc.HasLen, 2)
				c.Assert(nonceParts[0], gc.Equals, names.MachineTag("0"))
				c.Assert(nonceParts[1], checkers.Satisfies, utils.IsValidUUIDString)
				c.Assert(o.Secret, gc.Equals, secret)
				c.Assert(o.Constraints, gc.DeepEquals, cons)

				// Check we can connect to the state with
				// the machine's entity name and password.
				info := s.StateInfo(c)
				info.Tag = m.Tag()
				c.Assert(o.Info.Password, gc.Not(gc.HasLen), 0)
				info.Password = o.Info.Password
				c.Assert(o.Info, gc.DeepEquals, info)
				// Check we can connect to the state with
				// the machine's entity name and password.
				st, err := state.Open(o.Info, state.DefaultDialOpts())
				c.Assert(err, gc.IsNil)

				// All provisioned machines in this test suite have their hardware characteristics
				// attributes set to the same values as the constraints due to the dummy environment being used.
				hc, err := m.HardwareCharacteristics()
				c.Assert(err, gc.IsNil)
				c.Assert(*hc, gc.DeepEquals, instance.HardwareCharacteristics{
					Arch:     cons.Arch,
					Mem:      cons.Mem,
					RootDisk: cons.RootDisk,
					CpuCores: cons.CpuCores,
					CpuPower: cons.CpuPower,
				})
				st.Close()
				return
			default:
				c.Logf("ignoring unexpected operation %#v", o)
			}
		case <-time.After(2 * time.Second):
			c.Fatalf("provisioner did not start an instance")
			return
		}
	}
	return
}

// checkNoOperations checks that the environ was not operated upon.
func (s *CommonProvisionerSuite) checkNoOperations(c *gc.C) {
	s.State.StartSync()
	select {
	case o := <-s.op:
		c.Fatalf("unexpected operation %#v", o)
	case <-time.After(coretesting.ShortWait):
		return
	}
}

// checkStopInstances checks that an instance has been stopped.
func (s *CommonProvisionerSuite) checkStopInstances(c *gc.C, instances ...instance.Instance) {
	s.State.StartSync()
	instanceIds := set.NewStrings()
	for _, instance := range instances {
		instanceIds.Add(string(instance.Id()))
	}
	// Continue checking for stop instance calls until all the instances we
	// are waiting on to finish, actually finish, or we time out.
	for !instanceIds.IsEmpty() {
		select {
		case o := <-s.op:
			switch o := o.(type) {
			case dummy.OpStopInstances:
				for _, stoppedInstance := range o.Instances {
					instanceIds.Remove(string(stoppedInstance.Id()))
				}
			default:
				c.Fatalf("unexpected operation %#v", o)
				return
			}
		case <-time.After(2 * time.Second):
			c.Fatalf("provisioner did not stop an instance")
			return
		}
	}
}

func (s *CommonProvisionerSuite) waitMachine(c *gc.C, m *state.Machine, check func() bool) {
	// TODO(jam): We need to grow a new method on NotifyWatcherC
	// that calls StartSync while waiting for changes, then
	// waitMachine and waitHardwareCharacteristics can use that
	// instead
	w := m.Watch()
	defer stop(c, w)
	timeout := time.After(coretesting.LongWait)
	resync := time.After(0)
	for {
		select {
		case <-w.Changes():
			if check() {
				return
			}
		case <-resync:
			resync = time.After(coretesting.ShortWait)
			s.State.StartSync()
		case <-timeout:
			c.Fatalf("machine %v wait timed out", m)
		}
	}
}

func (s *CommonProvisionerSuite) waitHardwareCharacteristics(c *gc.C, m *state.Machine, check func() bool) {
	w := m.WatchHardwareCharacteristics()
	defer stop(c, w)
	timeout := time.After(coretesting.LongWait)
	resync := time.After(0)
	for {
		select {
		case <-w.Changes():
			if check() {
				return
			}
		case <-resync:
			resync = time.After(coretesting.ShortWait)
			s.State.StartSync()
		case <-timeout:
			c.Fatalf("hardware characteristics for machine %v wait timed out", m)
		}
	}
}

// waitRemoved waits for the supplied machine to be removed from state.
func (s *CommonProvisionerSuite) waitRemoved(c *gc.C, m *state.Machine) {
	s.waitMachine(c, m, func() bool {
		err := m.Refresh()
		if errors.IsNotFoundError(err) {
			return true
		}
		c.Assert(err, gc.IsNil)
		c.Logf("machine %v is still %s", m, m.Life())
		return false
	})
}

// waitInstanceId waits until the supplied machine has an instance id, then
// asserts it is as expected.
func (s *CommonProvisionerSuite) waitInstanceId(c *gc.C, m *state.Machine, expect instance.Id) {
	s.waitHardwareCharacteristics(c, m, func() bool {
		if actual, err := m.InstanceId(); err == nil {
			c.Assert(actual, gc.Equals, expect)
			return true
		} else if !state.IsNotProvisionedError(err) {
			// We don't expect any errors.
			panic(err)
		}
		c.Logf("machine %v is still unprovisioned", m)
		return false
	})
}

func (s *ProvisionerSuite) newEnvironProvisioner(machineId string) *provisioner.Provisioner {
	return provisioner.NewProvisioner(provisioner.ENVIRON, s.State, machineId, "")
}

func (s *ProvisionerSuite) TestProvisionerStartStop(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	c.Assert(p.Stop(), gc.IsNil)
}

func (s *ProvisionerSuite) addMachine() (*state.Machine, error) {
	params := state.AddMachineParams{
		Series:      config.DefaultSeries,
		Jobs:        []state.MachineJob{state.JobHostUnits},
		Constraints: s.defaultConstraints,
	}
	return s.State.AddMachineWithConstraints(&params)
}

func (s *ProvisionerSuite) TestSimple(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// Check that an instance is provisioned when the machine is created...
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	instance := s.checkStartInstance(c, m)

	// ...and removed, along with the machine, when the machine is Dead.
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, instance)
	s.waitRemoved(c, m)
}

func (s *ProvisionerSuite) TestConstraints(c *gc.C) {
	// Create a machine with non-standard constraints.
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	cons := constraints.MustParse("mem=8G arch=amd64 cpu-cores=2 root-disk=10G")
	err = m.SetConstraints(cons)
	c.Assert(err, gc.IsNil)

	// Start a provisioner and check those constraints are used.
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)
	s.checkStartInstanceCustom(c, m, "pork", cons)
}

func (s *ProvisionerSuite) TestProvisionerSetsErrorStatusWhenStartInstanceFailed(c *gc.C) {
	brokenMsg := breakDummyProvider(c, s.State, "StartInstance")
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// Check that an instance is not provisioned when the machine is created...
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	s.checkNoOperations(c)

	t0 := time.Now()
	for time.Since(t0) < coretesting.LongWait {
		// And check the machine status is set to error.
		status, info, err := m.Status()
		c.Assert(err, gc.IsNil)
		if status == params.StatusPending {
			time.Sleep(coretesting.ShortWait)
			continue
		}
		c.Assert(status, gc.Equals, params.StatusError)
		c.Assert(info, gc.Equals, brokenMsg)
		break
	}

	// Unbreak the environ config.
	err = s.fixEnvironment()
	c.Assert(err, gc.IsNil)

	// Restart the PA to make sure the machine is skipped again.
	stop(c, p)
	p = s.newEnvironProvisioner("0")
	defer stop(c, p)
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotOccurForContainers(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// create a machine to host the container.
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	inst := s.checkStartInstance(c, m)

	// make a container on the machine we just created
	params := state.AddMachineParams{
		ParentId:      m.Id(),
		ContainerType: instance.LXC,
		Series:        config.DefaultSeries,
		Jobs:          []state.MachineJob{state.JobHostUnits},
	}
	container, err := s.State.AddMachineWithConstraints(&params)
	c.Assert(err, gc.IsNil)

	// the PA should not attempt to create it
	s.checkNoOperations(c)

	// cleanup
	c.Assert(container.EnsureDead(), gc.IsNil)
	c.Assert(container.Remove(), gc.IsNil)
	c.Assert(m.EnsureDead(), gc.IsNil)
	s.checkStopInstances(c, inst)
	s.waitRemoved(c, m)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotOccurWithAnInvalidEnvironment(c *gc.C) {
	err := s.invalidateEnvironment(c)
	c.Assert(err, gc.IsNil)

	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// try to create a machine
	_, err = s.addMachine()
	c.Assert(err, gc.IsNil)

	// the PA should not create it
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestProvisioningOccursWithFixedEnvironment(c *gc.C) {
	err := s.invalidateEnvironment(c)
	c.Assert(err, gc.IsNil)

	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// try to create a machine
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)

	// the PA should not create it
	s.checkNoOperations(c)

	err = s.fixEnvironment()
	c.Assert(err, gc.IsNil)

	s.checkStartInstance(c, m)
}

func (s *ProvisionerSuite) TestProvisioningDoesOccurAfterInvalidEnvironmentPublished(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// place a new machine into the state
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)

	s.checkStartInstance(c, m)

	err = s.invalidateEnvironment(c)
	c.Assert(err, gc.IsNil)

	// create a second machine
	m, err = s.addMachine()
	c.Assert(err, gc.IsNil)

	// the PA should create it using the old environment
	s.checkStartInstance(c, m)
}

func (s *ProvisionerSuite) TestProvisioningDoesNotProvisionTheSameMachineAfterRestart(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// create a machine
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	s.checkStartInstance(c, m)

	// restart the PA
	stop(c, p)
	p = s.newEnvironProvisioner("0")
	defer stop(c, p)

	// check that there is only one machine known
	machines, err := p.AllMachines()
	c.Assert(err, gc.IsNil)
	c.Check(len(machines), gc.Equals, 1)
	c.Check(machines[0].Id(), gc.Equals, "0")

	// the PA should not create it a second time
	s.checkNoOperations(c)
}

func (s *ProvisionerSuite) TestProvisioningStopsInstances(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// create a machine
	m0, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	i0 := s.checkStartInstance(c, m0)

	// create a second machine
	m1, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	i1 := s.checkStartInstance(c, m1)
	stop(c, p)

	// mark the first machine as dead
	c.Assert(m0.EnsureDead(), gc.IsNil)

	// remove the second machine entirely
	c.Assert(m1.EnsureDead(), gc.IsNil)
	c.Assert(m1.Remove(), gc.IsNil)

	// start a new provisioner to shut them both down
	p = s.newEnvironProvisioner("0")
	defer stop(c, p)
	s.checkStopInstances(c, i0, i1)
	s.waitRemoved(c, m0)
}

func (s *ProvisionerSuite) TestDyingMachines(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// provision a machine
	m0, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	s.checkStartInstance(c, m0)

	// stop the provisioner and make the machine dying
	stop(c, p)
	err = m0.Destroy()
	c.Assert(err, gc.IsNil)

	// add a new, dying, unprovisioned machine
	m1, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	err = m1.Destroy()
	c.Assert(err, gc.IsNil)

	// start the provisioner and wait for it to reap the useless machine
	p = s.newEnvironProvisioner("0")
	defer stop(c, p)
	s.checkNoOperations(c)
	s.waitRemoved(c, m1)

	// verify the other one's still fine
	err = m0.Refresh()
	c.Assert(err, gc.IsNil)
	c.Assert(m0.Life(), gc.Equals, state.Dying)
}

func (s *ProvisionerSuite) TestProvisioningRecoversAfterInvalidEnvironmentPublished(c *gc.C) {
	p := s.newEnvironProvisioner("0")
	defer stop(c, p)

	// place a new machine into the state
	m, err := s.addMachine()
	c.Assert(err, gc.IsNil)
	s.checkStartInstance(c, m)

	err = s.invalidateEnvironment(c)
	c.Assert(err, gc.IsNil)
	s.State.StartSync()

	// create a second machine
	m, err = s.addMachine()
	c.Assert(err, gc.IsNil)

	// the PA should create it using the old environment
	s.checkStartInstance(c, m)

	err = s.fixEnvironment()
	c.Assert(err, gc.IsNil)

	// insert our observer
	cfgObserver := make(chan *config.Config, 1)
	p.SetObserver(cfgObserver)

	cfg, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	attrs := cfg.AllAttrs()
	attrs["secret"] = "beef"
	cfg, err = config.New(attrs)
	c.Assert(err, gc.IsNil)
	err = s.State.SetEnvironConfig(cfg)

	s.State.StartSync()

	// wait for the PA to load the new configuration
	select {
	case <-cfgObserver:
	case <-time.After(coretesting.LongWait):
		c.Fatalf("PA did not action config change")
	}

	// create a third machine
	m, err = s.addMachine()
	c.Assert(err, gc.IsNil)

	// the PA should create it using the new environment
	s.checkStartInstanceCustom(c, m, "beef", s.defaultConstraints)
}
