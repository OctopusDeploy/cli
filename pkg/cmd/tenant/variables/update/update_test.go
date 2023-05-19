package update_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/variables/update"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_ProjectVariable_AllFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "var name"
	flags.Project.Value = "project name"
	flags.Environment.Value = "dev environment"
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetTenantVariables = func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
		projectVars := variables.ProjectVariable{
			ProjectID:   "Projects-1",
			ProjectName: flags.Project.Value,
		}
		projectTemplate := actiontemplates.NewActionTemplateParameter()
		projectTemplate.Name = "var name"
		projectTemplate.DisplaySettings = make(map[string]string)
		projectTemplate.DisplaySettings["Octopus.ControlType"] = string(variables.ControlTypeSingleLineText)
		projectVars.Templates = append(projectVars.Templates, projectTemplate)
		vars := variables.NewTenantVariables("Tenants-1")
		vars.SpaceID = "Spaces-1"
		vars.TenantName = "tenant name"
		vars.ProjectVariables = make(map[string]variables.ProjectVariable)
		vars.ProjectVariables["Projects-1"] = projectVars
		return vars, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}

	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("tenant name"), nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_LibraryVariable_AllFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "var name"
	flags.LibraryVariableSet.Value = "lvs"
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetTenantVariables = func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
		libraryVariables := variables.NewLibraryVariable()
		libraryVariables.LibraryVariableSetID = "LibraryVariableSets-1"
		libraryVariables.LibraryVariableSetName = "lvs"
		template := actiontemplates.NewActionTemplateParameter()
		template.Name = "var name"
		template.DisplaySettings = make(map[string]string)
		template.DisplaySettings["Octopus.ControlType"] = string(variables.ControlTypeSingleLineText)
		libraryVariables.Templates = append(libraryVariables.Templates, template)
		vars := variables.NewTenantVariables("Tenants-1")
		vars.SpaceID = "Spaces-1"
		vars.TenantName = "tenant name"
		vars.LibraryVariables = make(map[string]variables.LibraryVariable)
		return vars, nil
	}

	opts.GetAllLibraryVariableSetsCallback = func() ([]*variables.LibraryVariableSet, error) {
		return []*variables.LibraryVariableSet{variables.NewLibraryVariableSet("lvs")}, nil
	}

	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("tenant name"), nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}
