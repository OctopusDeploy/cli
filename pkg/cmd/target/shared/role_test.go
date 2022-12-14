package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptRoles_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetRoleFlags()
	flags.Roles.Value = []string{"Man with hat"}

	opts := shared.NewCreateTargetRoleOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForRoles(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestPromptRolesAndEnvironments_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectWithAddPrompt("Choose at least one role for the deployment target.\n", "", []string{"Ninja #3", "Girl in crowd"}, []string{"Ninja #3"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetRoleFlags()

	opts := shared.NewCreateTargetRoleOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllRolesCallback = func() ([]string, error) {
		return []string{"Ninja #3", "Girl in crowd"}, nil
	}

	err := shared.PromptForRoles(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)

	assert.Equal(t, []string{"Ninja #3"}, flags.Roles.Value)
}
