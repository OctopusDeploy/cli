package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDistinctRoles_EmptyList(t *testing.T) {
	result := shared.DistinctRoles([]string{})
	assert.Empty(t, result)
}

func TestDistinctRoles_DuplicateValues(t *testing.T) {
	result := shared.DistinctRoles([]string{"a", "b", "a"})
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestPromptForTenant_FlagSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetCommonFlags()
	flags.TenantedDeploymentMode.Value = "Tenanted"
	flags.Tenants.Value = []string{"Tenant 1"}
	flags.TenantTags.Value = []string{"Tag Set 1/tag 1"}

	opts := shared.NewCreateTargetCommonOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForTenant(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestPromptForTenant_NoFlagsSupplied_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Choose the kind of deployments where this deployment target should be included", "",
			[]string{"Exclude from tenanted deployments (default)", "Include only in tenanted deployments", "Include in both tenanted and untenanted deployments"},
			"Include only in tenanted deployments"),
		testutil.NewMultiSelectPrompt("Select tenants this deployment target should be associated with", "",
			[]string{
				"Tenant 1",
				"Tag Set 1/tag 1",
			},
			[]string{
				"Tenant 1",
				"Tag Set 1/tag 1",
			}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetCommonFlags()

	opts := shared.NewCreateTargetCommonOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllTenantsCallback = func() ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{
			tenants.NewTenant("Tenant 1"),
		}, nil
	}

	opts.GetAllTagsCallback = func() ([]*tagsets.Tag, error) {
		tag := tagsets.NewTag("tag 1", "MissionBrown")
		tag.CanonicalTagName = "Tag Set 1/tag 1"
		return []*tagsets.Tag{tag}, nil
	}

	err := shared.PromptForTenant(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "Tenanted", flags.TenantedDeploymentMode.Value)
	assert.Equal(t, []string{"Tenant 1"}, flags.Tenants.Value)
	assert.Equal(t, []string{"Tag Set 1/tag 1"}, flags.TenantTags.Value)

}

func TestPromptRolesAndEnvironments_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetCommonFlags()
	flags.Environments.Value = []string{"Dev"}
	flags.Roles.Value = []string{"Man with hat"}

	opts := shared.NewCreateTargetCommonOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptRolesAndEnvironments(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestPromptRolesAndEnvironments_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Choose at least one environment for the deployment target.\n", "", []string{"Dev", "Test"}, []string{"Dev"}),
		testutil.NewMultiSelectWithAddPrompt("Choose at least one role for the deployment target.\n", "", []string{"Ninja #3", "Girl in crowd"}, []string{"Ninja #3"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetCommonFlags()

	opts := shared.NewCreateTargetCommonOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllRolesCallback = func() ([]string, error) {
		return []string{"Ninja #3", "Girl in crowd"}, nil
	}
	opts.GetAllEnvironmentsCallback = func() ([]*environments.Environment, error) {
		return []*environments.Environment{
			environments.NewEnvironment("Dev"),
			environments.NewEnvironment("Test"),
		}, nil
	}

	err := shared.PromptRolesAndEnvironments(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)

	assert.Equal(t, []string{"Dev"}, flags.Environments.Value)
	assert.Equal(t, []string{"Ninja #3"}, flags.Roles.Value)
}
