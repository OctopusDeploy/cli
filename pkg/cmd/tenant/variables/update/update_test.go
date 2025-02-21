package update_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/variables/update"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUpdate_CommonVariable_ScopeExactMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{"Dev", "Test"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		{
			Resource:               resources.Resource{ID: "TenantVariables-1"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 1"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:               resources.Resource{ID: "TenantVariables-2"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 2"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, nil, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(variablePayload))

	assert.Equal(t, existingCommonVariables[0].ID, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, existingCommonVariables[0].LibraryVariableSetId, variablePayload[0].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Equal(t, existingCommonVariables[0].Scope, variablePayload[0].Scope)

	assert.Equal(t, existingCommonVariables[1].ID, variablePayload[1].ID)
	assert.Equal(t, existingCommonVariables[1].Value, variablePayload[1].Value)
	assert.Equal(t, existingCommonVariables[1].LibraryVariableSetId, variablePayload[1].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[1].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingCommonVariables[1].Scope, variablePayload[1].Scope)
	assert.False(t, isSensitive)
}

func TestUpdate_CommonVariable_ScopePartialMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{"Dev"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		{
			Resource:               resources.Resource{ID: "TenantVariables-1"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 1"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:               resources.Resource{ID: "TenantVariables-2"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 2"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_CommonVariable_ScopeNoMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{"Dev", "Prod"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		{
			Resource:               resources.Resource{ID: "TenantVariables-1"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 1"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:               resources.Resource{ID: "TenantVariables-2"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 2"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_CommonVariable_WhenExistingValueIsMissingForScope(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{"Dev", "Test"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		{
			Resource:               resources.Resource{ID: "TenantVariables-2"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "existing value 2"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var missingCommonVariables = []variables.TenantCommonVariable{
		{
			Resource:               resources.Resource{ID: "TenantVariables-1"},
			LibraryVariableSetId:   "LibraryVariableSets-1",
			LibraryVariableSetName: "Set 1",
			TemplateID:             "Templates-1",
			Template:               *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:                  core.PropertyValue{Value: "default"},
			Scope:                  variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, missingCommonVariables, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(variablePayload))

	assert.Empty(t, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, missingCommonVariables[0].LibraryVariableSetId, variablePayload[0].LibraryVariableSetId)
	assert.Equal(t, missingCommonVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Equal(t, missingCommonVariables[0].Scope, variablePayload[0].Scope)

	assert.Equal(t, existingCommonVariables[0].ID, variablePayload[1].ID)
	assert.Equal(t, existingCommonVariables[0].Value, variablePayload[1].Value)
	assert.Equal(t, existingCommonVariables[0].LibraryVariableSetId, variablePayload[1].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[0].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingCommonVariables[0].Scope, variablePayload[1].Scope)
	assert.False(t, isSensitive)
}

func TestUpdate_ProjectVariable_ScopeExactMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{"Dev", "Test"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		{
			Resource:    resources.Resource{ID: "TenantVariables-1"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 1"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:    resources.Resource{ID: "TenantVariables-2"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 2"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, nil, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(variablePayload))

	assert.Equal(t, existingProjectVariables[0].ID, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, existingProjectVariables[0].ProjectID, variablePayload[0].ProjectID)
	assert.Equal(t, existingProjectVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Equal(t, existingProjectVariables[0].Scope, variablePayload[0].Scope)

	assert.Equal(t, existingProjectVariables[1].ID, variablePayload[1].ID)
	assert.Equal(t, existingProjectVariables[1].Value, variablePayload[1].Value)
	assert.Equal(t, existingProjectVariables[1].ProjectID, variablePayload[1].ProjectID)
	assert.Equal(t, existingProjectVariables[1].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingProjectVariables[1].Scope, variablePayload[1].Scope)
	assert.False(t, isSensitive)
}

func TestUpdate_ProjectVariable_ScopePartialMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{"Dev"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		{
			Resource:    resources.Resource{ID: "TenantVariables-1"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 1"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:    resources.Resource{ID: "TenantVariables-2"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 2"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_ProjectVariable_ScopeNoMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{"Dev", "Prod"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		{
			Resource:    resources.Resource{ID: "TenantVariables-1"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 1"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
		{
			Resource:    resources.Resource{ID: "TenantVariables-2"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 2"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_ProjectVariable_WhenExistingValueIsMissingForScope(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{"Dev", "Test"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		{
			Resource:    resources.Resource{ID: "TenantVariables-2"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "existing value 2"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-3"}},
		},
	}

	var missingProjectVariables = []variables.TenantProjectVariable{
		{
			Resource:    resources.Resource{ID: "TenantVariables-1"},
			ProjectID:   "Projects-1",
			ProjectName: "Project 1",
			TemplateID:  "Templates-1",
			Template:    *createTemplate("Template 1", resources.ControlTypeSingleLineText, "Templates-1"),
			Value:       core.PropertyValue{Value: "default"},
			Scope:       variables.TenantVariableScope{EnvironmentIds: []string{"Environments-1", "Environments-2"}},
		},
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, missingProjectVariables, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(variablePayload))

	assert.Empty(t, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, missingProjectVariables[0].ProjectID, variablePayload[0].ProjectID)
	assert.Equal(t, missingProjectVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Equal(t, missingProjectVariables[0].Scope, variablePayload[0].Scope)

	assert.Equal(t, existingProjectVariables[0].ID, variablePayload[1].ID)
	assert.Equal(t, existingProjectVariables[0].Value, variablePayload[1].Value)
	assert.Equal(t, existingProjectVariables[0].ProjectID, variablePayload[1].ProjectID)
	assert.Equal(t, existingProjectVariables[0].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingProjectVariables[0].Scope, variablePayload[1].Scope)
	assert.False(t, isSensitive)
}

func TestPromptMissing_ProjectVariable_AllFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "var name"
	flags.Project.Value = "project name"
	flags.Environments.Value = []string{"dev environment"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetTenantVariables = func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
		projectVars := variables.ProjectVariable{
			ProjectID:   "Projects-1",
			ProjectName: flags.Project.Value,
		}
		projectTemplate := createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1")
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
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1"))
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
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1"))
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name 2", resources.ControlTypeSingleLineText, "templateId-2"))
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
		project1Vars.Templates = append(project1Vars.Templates, createTemplate("project 1 var", resources.ControlTypeSingleLineText, "templateId-1"))
		project1Vars.Variables["Environments-1"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-1"]["templateId-1"] = core.NewPropertyValue("", false)
		project1Vars.Variables["Environments-2"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-2"]["templateId-1"] = core.NewPropertyValue("", false)
		project2Vars := variables.ProjectVariable{
			ProjectID:   "Projects-2",
			ProjectName: "Project 2",
			Variables:   make(map[string]map[string]core.PropertyValue),
		}
		project2Vars.Templates = append(project2Vars.Templates, createTemplate("project 2 var", resources.ControlTypeSingleLineText, "templateId-2"))
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

func createTemplate(name string, controlType resources.ControlType, templateId string) *actiontemplates.ActionTemplateParameter {
	template := actiontemplates.NewActionTemplateParameter()
	template.ID = templateId
	template.Name = name
	template.DisplaySettings = make(map[string]string)
	template.DisplaySettings["Octopus.ControlType"] = string(controlType)
	return template
}
