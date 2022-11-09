package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptEnvironments_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetEnvironmentFlags()
	flags.Environments.Value = []string{"Dev"}

	opts := shared.NewCreateTargetEnvironmentOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForEnvironments(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestPromptEnvironments_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Choose at least one environment for the deployment target.\n", "", []string{"Dev", "Test"}, []string{"Dev"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetEnvironmentFlags()

	opts := shared.NewCreateTargetEnvironmentOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllEnvironmentsCallback = func() ([]*environments.Environment, error) {
		return []*environments.Environment{
			environments.NewEnvironment("Dev"),
			environments.NewEnvironment("Test"),
		}, nil
	}

	err := shared.PromptForEnvironments(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)

	assert.Equal(t, []string{"Dev"}, flags.Environments.Value)
}
