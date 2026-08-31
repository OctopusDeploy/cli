package shared_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
)

func TestPromptMissingTarget_IdentifierSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := shared.NewSetDisabledStateOptions([]string{"Machines-1"}, &cmd.Dependencies{Ask: asker})

	err := shared.PromptMissingTarget(opts, true)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

func TestPromptMissingTarget_NoIdentifierSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to disable:", "", []string{"web-server", "db-server"}, "db-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := shared.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker})
	opts.GetTargetsCallback = func() ([]*machines.DeploymentTarget, error) {
		return []*machines.DeploymentTarget{
			newTestTarget("Machines-1", "web-server"),
			newTestTarget("Machines-2", "db-server"),
		}, nil
	}

	err := shared.PromptMissingTarget(opts, true)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-2", opts.IdOrName)
}

func TestPromptMissingTarget_EnableUsesEnableWording(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to enable:", "", []string{"web-server", "db-server"}, "web-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := shared.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker})
	opts.GetTargetsCallback = func() ([]*machines.DeploymentTarget, error) {
		return []*machines.DeploymentTarget{
			newTestTarget("Machines-1", "web-server"),
			newTestTarget("Machines-2", "db-server"),
		}, nil
	}

	err := shared.PromptMissingTarget(opts, false)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

// selectors.Select auto-selects when there is exactly one item; enable/disable must not do that
// because it would mutate the only target in the space without asking.
func TestPromptMissingTarget_AsksEvenWhenThereIsOnlyOneTarget(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the deployment target you wish to disable:", "", []string{"web-server"}, "web-server"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := shared.NewSetDisabledStateOptions([]string{}, &cmd.Dependencies{Ask: asker})
	opts.GetTargetsCallback = func() ([]*machines.DeploymentTarget, error) {
		return []*machines.DeploymentTarget{newTestTarget("Machines-1", "web-server")}, nil
	}

	err := shared.PromptMissingTarget(opts, true)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Machines-1", opts.IdOrName)
}

func newTestTarget(id string, name string) *machines.DeploymentTarget {
	target := machines.NewDeploymentTarget(name, machines.NewCloudRegionEndpoint(), []string{"Environments-1"}, []string{"web"})
	target.ID = id
	target.SpaceID = "Spaces-1"
	return target
}
