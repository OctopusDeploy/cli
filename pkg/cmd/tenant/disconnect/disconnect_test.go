package disconnect_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/disconnect"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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

	flags := disconnect.NewDisconnectFlags()
	flags.Tenant.Value = "Test Tenant"
	flags.Project.Value = "Project 1"
	flags.Confirm.Value = true

	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetTenantCallback = func(id string) (*tenants.Tenant, error) {
		return tenants.NewTenant(flags.Tenant.Value), nil
	}

	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		return projects.NewProject(flags.Project.Value, "Lifecycles-1", "ProjectGroups-1"), nil
	}

	err := disconnect.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForProject_ZeroProjectsConnected(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	tenant := tenants.NewTenant("Test Tenant")
	tenant.ProjectEnvironments = make(map[string][]string)

	flags := disconnect.NewDisconnectFlags()
	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{Ask: asker})

	project, err := disconnect.PromptForProject(opts, tenant)
	checkRemainingPrompts()
	assert.Error(t, err, "Not currently connected to any projects")
	assert.Nil(t, project)
}

func TestPromptForProject_OneProjectConnected(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	project := projects.NewProject("The project", "Lifecycles-1", "ProjectGroups-1")
	project.ID = "Projects-1"
	tenant := tenants.NewTenant("Test Tenant")
	tenant.ProjectEnvironments = make(map[string][]string)
	tenant.ProjectEnvironments[project.ID] = []string{"Environments-1"}

	flags := disconnect.NewDisconnectFlags()
	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectCallback = func(id string) (*projects.Project, error) { return project, nil }

	selectedProject, err := disconnect.PromptForProject(opts, tenant)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, project, selectedProject)
	assert.Equal(t, project.Name, opts.Project.Value)
}

func TestPromptForProject_MultipleProjectsConnected(t *testing.T) {
	project1 := projects.NewProject("Project 1", "Lifecycles-1", "ProjectGroups-1")
	project1.ID = "Projects-1"
	project2 := projects.NewProject("Project 2", "Lifecycles-1", "ProjectGroups-1")
	project2.ID = "Projects-2"
	tenant := tenants.NewTenant("Test Tenant")
	tenant.ProjectEnvironments = make(map[string][]string)
	tenant.ProjectEnvironments[project1.ID] = []string{"Environments-1"}
	tenant.ProjectEnvironments[project2.ID] = []string{"Environments-1"}
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Project. Please select one:", "", []string{project1.GetName(), project2.GetName()}, project1.GetName()),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := disconnect.NewDisconnectFlags()
	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		if id == "Projects-1" {
			return project1, nil
		} else {
			return project2, nil
		}
	}

	selectedProject, err := disconnect.PromptForProject(opts, tenant)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Project 1", opts.Project.Value)
	assert.Equal(t, selectedProject, project1)
}

func TestDisconnectRun_NoConfirmation_ShouldError(t *testing.T) {
	flags := disconnect.NewDisconnectFlags()
	flags.Confirm.Value = false

	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{})
	opts.NoPrompt = true

	err := disconnect.DisconnectRun(opts)
	assert.Error(t, err, "Cannot disconnect without confirmation")
}

func TestDisconnectRun_NotConnectedToAnyProjects_ShouldError(t *testing.T) {
	flags := disconnect.NewDisconnectFlags()
	flags.Confirm.Value = true

	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{})
	opts.NoPrompt = true

	tenant := tenants.NewTenant("Test Tenant")
	tenant.ProjectEnvironments = make(map[string][]string)
	opts.GetProjectCallback = func(id string) (*projects.Project, error) { return nil, nil }
	opts.GetTenantCallback = func(id string) (*tenants.Tenant, error) { return tenant, nil }

	err := disconnect.DisconnectRun(opts)
	assert.Error(t, err, "Tenant is not currently connected to any projects")
}

func TestDisconnectRun_NotConnectedToProject_ShouldError(t *testing.T) {
	flags := disconnect.NewDisconnectFlags()
	flags.Confirm.Value = true
	flags.Project.Value = "disconnected"

	opts := disconnect.NewDisconnectOptions(flags, &cmd.Dependencies{})
	opts.NoPrompt = true

	tenant := tenants.NewTenant("Test Tenant")
	tenant.ProjectEnvironments = make(map[string][]string)
	tenant.ProjectEnvironments["Projects-100"] = []string{"Environments-1"}
	opts.GetProjectCallback = func(id string) (*projects.Project, error) {
		return projects.NewProject(flags.Project.Value, "Lc-1", "pg-1"), nil
	}
	opts.GetTenantCallback = func(id string) (*tenants.Tenant, error) { return tenant, nil }

	err := disconnect.DisconnectRun(opts)
	assert.Error(t, err, "Tenant is not currently connected to the 'disconnected' project")
}
