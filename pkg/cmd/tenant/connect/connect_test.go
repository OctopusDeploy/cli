package connect_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/connect"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("https://serverurl")
var spinner = &testutil.FakeSpinner{}
var rootResource = testutil.NewRootResource()

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := connect.NewConnectFlags()
	flags.Tenant.Value = "Tennents"
	flags.Project.Value = "Stella Artois"
	flags.Environments.Value = []string{"Drouthy Neebors"}

	opts := connect.NewConnectOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllTenantsCallback = func() ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{
			tenants.NewTenant(flags.Tenant.Value),
		}, nil
	}
	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		project := projects.NewProject(flags.Project.Name, "Lifecycles-1", "ProjectGroups-1")
		project.TenantedDeploymentMode = core.TenantedDeploymentModeTenantedOrUntenanted
		return project, nil
	}

	err := connect.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_ProjectSupportsTenants(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Tenant. Please select one:", "", []string{"Tenant 1", "Tenant 2"}, "Tenant 1"),
		testutil.NewSelectPrompt("You have not specified a Project. Please select one:", "", []string{"Project A", "Project B"}, "Project A"),
		testutil.NewMultiSelectPrompt("You have not specified any environments. Please select at least one:", "", []string{"Env 1", "Env 2", "Env 3"}, []string{"Env 1", "Env 3"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := connect.NewConnectFlags()
	opts := connect.NewConnectOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllTenantsCallback = func() ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{
			tenants.NewTenant("Tenant 1"),
			tenants.NewTenant("Tenant 2"),
		}, nil
	}
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{
			projects.NewProject("Project A", "Lifecycles-1", "ProjectGroups-1"),
			projects.NewProject("Project B", "Lifecycles-1", "ProjectGroups-1"),
		}, nil
	}

	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		project := projects.NewProject("Project A", "Lifecycles-1", "ProjectGroups-1")
		project.TenantedDeploymentMode = core.TenantedDeploymentModeTenantedOrUntenanted
		return project, nil
	}
	opts.GetProjectProgressionCallback = func(project *projects.Project) (*projects.Progression, error) {
		return &projects.Progression{
			Environments: []*resources.ReferenceDataItem{
				{ID: "Environments-1", Name: "Env 1"},
				{ID: "Environments-2", Name: "Env 2"},
				{ID: "Environments-3", Name: "Env 3"},
			},
		}, nil
	}

	err := connect.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Tenant 1", opts.Tenant.Value)
	assert.Equal(t, "Project A", opts.Project.Value)
	assert.Equal(t, []string{"Env 1", "Env 3"}, opts.Environments.Value)
}

func TestPromptForEnablingTenantedDeployments_AnswerYes_ShouldError(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("Do you want to enable tenanted deployments for 'Project A'?", "", true),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := connect.NewConnectFlags()
	flags.EnableTenantDeployments.Value = false
	opts := connect.NewConnectOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		project := projects.NewProject("Project A", "Lifecycles-1", "ProjectGroups-1")
		project.TenantedDeploymentMode = core.TenantedDeploymentModeUntenanted
		return project, nil
	}

	err := connect.PromptForEnablingTenantedDeployments(opts, opts.GetProjectCallback)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.True(t, opts.EnableTenantDeployments.Value)
}

func TestPromptForEnablingTenantedDeployments_AnswerNo_ShouldError(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("Do you want to enable tenanted deployments for 'Project A'?", "", false),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := connect.NewConnectFlags()
	flags.EnableTenantDeployments.Value = false
	opts := connect.NewConnectOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		project := projects.NewProject("Project A", "Lifecycles-1", "ProjectGroups-1")
		project.TenantedDeploymentMode = core.TenantedDeploymentModeUntenanted
		return project, nil
	}

	err := connect.PromptForEnablingTenantedDeployments(opts, opts.GetProjectCallback)
	checkRemainingPrompts()
	assert.Error(t, err, "Cannot connect tenant to 'Project A' as it does not support tenanted deployments.")
}
