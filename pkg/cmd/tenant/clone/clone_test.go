package clone_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/clone"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_AllFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := clone.NewCloneFlags()
	flags.Name.Value = "Cloned tenant"
	flags.Description.Value = "the description"
	flags.SourceTenant.Value = "source tenant"

	opts := clone.NewCloneOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("source tenant"), nil
	}

	err := clone.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_NoFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this Tenant.",  "cloned tenant"),
		testutil.NewInputPrompt("Description", "A short, memorable, description for this Tenant.", "the description"),
		testutil.NewSelectPrompt("You have not specified a source Tenant to clone from. Please select one:", "", []string{"source tenant", "source tenant 2"}, "source tenant"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := clone.NewCloneFlags()
	opts := clone.NewCloneOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("source tenant"), nil
	}
	opts.GetAllTenantsCallback= func () ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{
		 tenants.NewTenant("source tenant"),
		 tenants.NewTenant("source tenant 2"),
		},nil
	}

	err := clone.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)

	assert.Equal(t, "cloned tenant", flags.Name.Value)
	assert.Equal(t, "the description", flags.Description.Value)
	assert.Equal(t, "source tenant", flags.SourceTenant.Value)
}