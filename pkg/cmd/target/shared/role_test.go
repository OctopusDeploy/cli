package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDistinctRoles_EmptyList(t *testing.T) {
	result := util.SliceDistinct([]string{})
	assert.Empty(t, result)
}

func TestDistinctRoles_DuplicateValues(t *testing.T) {
	result := util.SliceDistinct([]string{"a", "b", "a"})
	assert.Equal(t, []string{"a", "b"}, result)
}

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

func TestValidateTags_EmptyList(t *testing.T) {
	result, err := shared.ValidateTags(nil, []string{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestValidateTags_InvalidFormat_NoSlash(t *testing.T) {
	_, err := shared.ValidateTags(nil, []string{"PlainName"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not in the correct format")
}

func TestValidateTags_InvalidFormat_MultipleSlashes(t *testing.T) {
	_, err := shared.ValidateTags(nil, []string{"Set/Tag/Extra"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to any tag set")
}

func TestCombineRolesAndTags_RolesOnly(t *testing.T) {
	result, err := shared.CombineRolesAndTags(nil, []string{"web-server", "db-server"}, []string{})
	assert.NoError(t, err)
	assert.Equal(t, []string{"web-server", "db-server"}, result)
}

func TestCombineRolesAndTags_EmptyBoth(t *testing.T) {
	result, err := shared.CombineRolesAndTags(nil, []string{}, []string{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestCombineRolesAndTags_TagValidationError(t *testing.T) {
	_, err := shared.CombineRolesAndTags(nil, []string{"web-server"}, []string{"InvalidFormat"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not in the correct format")
}
