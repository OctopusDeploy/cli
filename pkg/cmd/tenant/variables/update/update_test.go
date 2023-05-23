package update_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/variables/update"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
		projectTemplate := createTemplate("var name", variables.ControlTypeSingleLineText, "templateId-1")
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
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", variables.ControlTypeSingleLineText, "templateId-1"))
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

func TestPromptMissing_LibraryVariable_NoFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Tenant. Please select one:", "", []string{"tenant name", "tenant name 2"}, "tenant name"),
		testutil.NewSelectPrompt("Which type of variable do you want to update?", "", []string{"Library/Common", "Project"}, "Library/Common"),
		testutil.NewSelectPrompt("You have not specified a variable", "", []string{"lvs / var name", "lvs / var name 2"}, "lvs / var name"),
		testutil.NewInputPrompt("Value", "", "var value"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantVariables = func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
		libraryVariables := variables.NewLibraryVariable()
		libraryVariables.LibraryVariableSetID = "LibraryVariableSets-1"
		libraryVariables.LibraryVariableSetName = "lvs"
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", variables.ControlTypeSingleLineText, "templateId-1"))
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name 2", variables.ControlTypeSingleLineText, "templateId-2"))
		vars := variables.NewTenantVariables("Tenants-1")
		vars.SpaceID = "Spaces-1"
		vars.TenantName = "tenant name"
		vars.LibraryVariables = make(map[string]variables.LibraryVariable)
		vars.LibraryVariables[libraryVariables.LibraryVariableSetID] = *libraryVariables
		return vars, nil
	}

	opts.GetAllLibraryVariableSetsCallback = func() ([]*variables.LibraryVariableSet, error) {
		return []*variables.LibraryVariableSet{variables.NewLibraryVariableSet("lvs")}, nil
	}

	opts.GetAllTenantsCallback = func() ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{tenants.NewTenant("tenant name"), tenants.NewTenant("tenant name 2")}, nil
	}
	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("tenant name"), nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.Equal(t, "var value", flags.Value.Value)
	assert.Equal(t, "lvs", flags.LibraryVariableSet.Value)
	assert.Empty(t, flags.Project.Value)
	assert.Equal(t, "var name", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func TestPromptMissing_ProjectVariable_NoFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Tenant. Please select one:", "", []string{"tenant name", "tenant name 2"}, "tenant name"),
		testutil.NewSelectPrompt("Which type of variable do you want to update?", "", []string{"Library/Common", "Project"}, "Project"),
		testutil.NewSelectPrompt("You have not specified a variable", "", []string{"Project 1 / project 1 var", "Project 2 / project 2 var"}, "Project 2 / project 2 var"),
		testutil.NewSelectPrompt("You have not specified an environment", "", []string{"Staging", "Production"}, "Staging"),
		testutil.NewInputPrompt("Value", "", "var value"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantVariables = func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
		project1Vars := variables.ProjectVariable{
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			Variables:   make(map[string]map[string]core.PropertyValue),
		}
		project1Vars.Templates = append(project1Vars.Templates, createTemplate("project 1 var", variables.ControlTypeSingleLineText, "templateId-1"))
		project1Vars.Variables["Environments-1"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-1"]["templateId-1"] = core.NewPropertyValue("", false)
		project1Vars.Variables["Environments-2"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-2"]["templateId-1"] = core.NewPropertyValue("", false)
		project2Vars := variables.ProjectVariable{
			ProjectID:   "Projects-2",
			ProjectName: "Project 2",
			Variables:   make(map[string]map[string]core.PropertyValue),
		}
		project2Vars.Templates = append(project2Vars.Templates, createTemplate("project 2 var", variables.ControlTypeSingleLineText, "templateId-2"))
		project2Vars.Variables["Environments-1"] = make(map[string]core.PropertyValue)
		project2Vars.Variables["Environments-1"]["templateId-2"] = core.NewPropertyValue("", false)
		project2Vars.Variables["Environments-2"] = make(map[string]core.PropertyValue)
		project2Vars.Variables["Environments-2"]["templateId-2"] = core.NewPropertyValue("", false)

		vars := variables.NewTenantVariables("Tenants-1")
		vars.SpaceID = "Spaces-1"
		vars.TenantName = "tenant name"
		vars.ProjectVariables = make(map[string]variables.ProjectVariable)
		vars.ProjectVariables["Projects-1"] = project1Vars
		vars.ProjectVariables["Projects-2"] = project2Vars

		return vars, nil
	}
	opts.GetAllLibraryVariableSetsCallback = func() ([]*variables.LibraryVariableSet, error) {
		return []*variables.LibraryVariableSet{variables.NewLibraryVariableSet("lvs")}, nil
	}

	opts.GetAllTenantsCallback = func() ([]*tenants.Tenant, error) {
		return []*tenants.Tenant{tenants.NewTenant("tenant name"), tenants.NewTenant("tenant name 2")}, nil
	}
	opts.GetTenantCallback = func(identifier string) (*tenants.Tenant, error) {
		return tenants.NewTenant("tenant name"), nil
	}
	opts.GetAllEnvironmentsCallback = func() ([]*environments.Environment, error) {
		staging := environments.NewEnvironment("Staging")
		staging.ID = "Environments-1"
		production := environments.NewEnvironment("Production")
		production.ID = "Environments-2"
		return []*environments.Environment{staging, production}, nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.Equal(t, "var value", flags.Value.Value)
	assert.Empty(t, flags.LibraryVariableSet.Value)
	assert.Equal(t, "Project 2", flags.Project.Value)
	assert.Equal(t, "project 2 var", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func createTemplate(name string, controlType variables.ControlType, templateId string) *actiontemplates.ActionTemplateParameter {
	template := actiontemplates.NewActionTemplateParameter()
	template.Name = name
	template.DisplaySettings = make(map[string]string)
	template.DisplaySettings["Octopus.ControlType"] = string(controlType)
	return template
}
