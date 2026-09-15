package machinescommon_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
)

// fakeMachine stands in for a deployment target or a worker; the real adapters are covered
// end-to-end by the deployment-target and worker enable/disable command tests.
type fakeMachine struct {
	id       string
	name     string
	disabled bool
}

func (m *fakeMachine) ID() string                { return m.id }
func (m *fakeMachine) Name() string              { return m.name }
func (m *fakeMachine) IsDisabled() bool          { return m.disabled }
func (m *fakeMachine) SetDisabled(disabled bool) { m.disabled = disabled }
func (m *fakeMachine) Save() error               { return nil }

func kindReturning(noun string, machines ...machinescommon.Machine) *machinescommon.MachineKind {
	return &machinescommon.MachineKind{
		Noun: noun,
		GetAllInState: func(isDisabled bool) ([]machinescommon.Machine, error) {
			return machines, nil
		},
	}
}

func TestPromptMissingMachine_IdentifierSupplied(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})
	opts := machinescommon.NewSetDisabledStateOptions([]string{"Machines-1"}, &cmd.Dependencies{Ask: asker}, kindReturning("deployment target"), true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

func TestPromptMissingMachine_NoIdentifierSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to disable:", "", []string{"web-server", "db-server"}, "db-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	kind := kindReturning("deployment target",
		&fakeMachine{id: "Machines-1", name: "web-server"},
		&fakeMachine{id: "Machines-2", name: "db-server"})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-2", opts.IdOrName)
}

func TestPromptMissingMachine_EnableUsesEnableWording(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to enable:", "", []string{"web-server", "db-server"}, "web-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	kind := kindReturning("deployment target",
		&fakeMachine{id: "Machines-1", name: "web-server", disabled: true},
		&fakeMachine{id: "Machines-2", name: "db-server", disabled: true})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, false)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

// The noun comes from the kind, so workers get worker wording for free.
func TestPromptMissingMachine_WorkerUsesWorkerWording(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the worker you wish to disable:", "", []string{"build-worker"}, "build-worker"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	kind := kindReturning("worker", &fakeMachine{id: "Workers-1", name: "build-worker"})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Workers-1", opts.IdOrName)
}

// selectors.Select auto-selects when there is exactly one item; enable/disable must not do that
// because it would mutate the only machine in the space without asking.
func TestPromptMissingMachine_AsksEvenWhenThereIsOnlyOneMachine(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to disable:", "", []string{"web-server"}, "web-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	kind := kindReturning("deployment target", &fakeMachine{id: "Machines-1", name: "web-server"})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

func TestPromptMissingMachine_ErrorsWhenNothingIsInTheOppositeState(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})
	kind := kindReturning("deployment target", &fakeMachine{id: "Machines-1", name: "web-server", disabled: true})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.EqualError(t, err, "no deployment targets to disable were found")
}

func TestPromptMissingMachine_ErrorsWhenNoWorkerIsInTheOppositeState(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})
	kind := kindReturning("worker", &fakeMachine{id: "Workers-1", name: "build-worker", disabled: true})
	opts := machinescommon.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker}, kind, true)

	err := machinescommon.PromptMissingMachine(opts)
	checkRemainingPrompts()

	assert.EqualError(t, err, "no workers to disable were found")
}
