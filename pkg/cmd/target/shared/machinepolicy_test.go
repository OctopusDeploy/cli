package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMachinePolicyFlagSupplied_ShouldNotPrompt(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetMachinePolicyFlags()
	flags.MachinePolicy.Value = "MachinePolicy-1"

	opts := shared.NewCreateTargetMachinePolicyOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForMachinePolicy(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestNoFlag_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the machine policy to use", "", []string{"Policy 1", "Policy 2"}, "Policy 2"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetMachinePolicyFlags()
	opts := shared.NewCreateTargetMachinePolicyOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllMachinePoliciesCallback = func() ([]*machines.MachinePolicy, error) {
		return []*machines.MachinePolicy{
			machines.NewMachinePolicy("Policy 1"),
			machines.NewMachinePolicy("Policy 2"),
		}, nil
	}

	err := shared.PromptForMachinePolicy(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Policy 2", flags.MachinePolicy.Value)
}
