package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/account/generic-oidc/create"
	"github.com/OctopusDeploy/cli/test/testutil"
)

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "The Final Frontier"
	flags.Description.Value = "Where no person has gone before"
	flags.ExecutionSubjectKeys.Value = []string{"space"}
	flags.Audience.Value = "custom audience"
	flags.Environments.Value = []string{"dev"}

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
	}
	_ = create.PromptMissing(opts)
	checkRemainingPrompts()
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this account.", "oidc account"),
		testutil.NewMultiSelectPrompt("Deployment and Runbook subject keys", "", []string{"space", "environment", "project", "tenant", "runbook", "account", "type"}, []string{"space", "type"}),
		testutil.NewInputPromptWithDefault("Audience", "Set this if you need to override the default Audience value.", "api://default", "custom audience"),
		testutil.NewMultiSelectPrompt("Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.", "", []string{"testenv"}, []string{"testenv"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Description.Value = "the description" // this is due the input mocking not support OctoEditor

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{fixtures.NewEnvironment("Spaces-1", "Environments-1", "testenv")}, nil
		},
	}
	_ = create.PromptMissing(opts)

	assert.Equal(t, "oidc account", flags.Name.Value)
	assert.Equal(t, "custom audience", flags.Audience.Value)
	assert.Equal(t, []string{"Environments-1"}, flags.Environments.Value)
	checkRemainingPrompts()
}
