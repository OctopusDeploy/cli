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

func TestUpdate_UnscopedCommonVariable_ScopeExactMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{}, false),
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_UnscopedCommonVariable_ScopeNoMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.LibraryVariableSet.Value = "Set 1"
	flags.Environments.Value = []string{}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingCommonVariables = []variables.TenantCommonVariable{
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateCommonVariableValue(opts, existingCommonVariables, nil, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(variablePayload))

	assert.Empty(t, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, existingCommonVariables[0].LibraryVariableSetId, variablePayload[0].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Empty(t, variablePayload[0].Scope.EnvironmentIds)

	assert.Equal(t, existingCommonVariables[0].ID, variablePayload[1].ID)
	assert.Equal(t, existingCommonVariables[0].Value, variablePayload[1].Value)
	assert.Equal(t, existingCommonVariables[0].LibraryVariableSetId, variablePayload[1].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[0].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingCommonVariables[0].Scope, variablePayload[1].Scope)

	assert.Equal(t, existingCommonVariables[1].ID, variablePayload[2].ID)
	assert.Equal(t, existingCommonVariables[1].Value, variablePayload[2].Value)
	assert.Equal(t, existingCommonVariables[1].LibraryVariableSetId, variablePayload[2].LibraryVariableSetId)
	assert.Equal(t, existingCommonVariables[1].TemplateID, variablePayload[2].TemplateID)
	assert.Equal(t, existingCommonVariables[1].Scope, variablePayload[2].Scope)
	assert.False(t, isSensitive)
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
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var missingCommonVariables = []variables.TenantCommonVariable{
		createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "", []string{"Environments-1", "Environments-2"}, false),
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

func TestUpdate_UnscopedProjectVariable_ScopeExactMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{}, false),
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	_, _, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, nil, environmentMap)

	assert.Error(t, err)
}

func TestUpdate_UnscopedProjectVariable_ScopeNoMatch(t *testing.T) {
	pa := []*testutil.PA{}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Tenant.Value = "tenant name"
	flags.Value.Value = "new value"
	flags.Name.Value = "Template 1"
	flags.Project.Value = "Project 1"
	flags.Environments.Value = []string{}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	var existingProjectVariables = []variables.TenantProjectVariable{
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var environmentMap = map[string]string{
		"Environments-1": "Dev",
		"Environments-2": "Test",
		"Environments-3": "Prod",
	}

	isSensitive, variablePayload, err := update.UpdateProjectVariableValue(opts, existingProjectVariables, nil, environmentMap)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(variablePayload))

	assert.Empty(t, variablePayload[0].ID)
	assert.Equal(t, core.PropertyValue{Value: "new value"}, variablePayload[0].Value)
	assert.Equal(t, existingProjectVariables[0].ProjectID, variablePayload[0].ProjectID)
	assert.Equal(t, existingProjectVariables[0].TemplateID, variablePayload[0].TemplateID)
	assert.Empty(t, variablePayload[0].Scope.EnvironmentIds)

	assert.Equal(t, existingProjectVariables[0].ID, variablePayload[1].ID)
	assert.Equal(t, existingProjectVariables[0].Value, variablePayload[1].Value)
	assert.Equal(t, existingProjectVariables[0].ProjectID, variablePayload[1].ProjectID)
	assert.Equal(t, existingProjectVariables[0].TemplateID, variablePayload[1].TemplateID)
	assert.Equal(t, existingProjectVariables[0].Scope, variablePayload[1].Scope)

	assert.Equal(t, existingProjectVariables[1].ID, variablePayload[2].ID)
	assert.Equal(t, existingProjectVariables[1].Value, variablePayload[2].Value)
	assert.Equal(t, existingProjectVariables[1].ProjectID, variablePayload[2].ProjectID)
	assert.Equal(t, existingProjectVariables[1].TemplateID, variablePayload[2].TemplateID)
	assert.Equal(t, existingProjectVariables[1].Scope, variablePayload[2].Scope)
	assert.False(t, isSensitive)
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
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1", "Environments-2"}, false),
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
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
		createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
	}

	var missingProjectVariables = []variables.TenantProjectVariable{
		createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "", []string{"Environments-1", "Environments-2"}, false),
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

	opts.GetTenantProjectVariables = func(tenant *tenants.Tenant, includeMissingVariables bool) (*variables.GetTenantProjectVariablesResponse, error) {
		return &variables.GetTenantProjectVariablesResponse{
			TenantID: "Tenants-1",
			ProjectVariables: []variables.TenantProjectVariable{
				createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
			},
		}, nil
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
	flags.Environments.Value = []string{"dev environment"}
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantCommonVariables = func(tenant *tenants.Tenant, includeMissingVariables bool) (*variables.GetTenantCommonVariablesResponse, error) {
		return &variables.GetTenantCommonVariablesResponse{
			TenantID: "Tenants-1",
			CommonVariables: []variables.TenantCommonVariable{
				createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-3"}, false),
			},
		}, nil
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
		testutil.NewSelectPrompt("You have not specified a variable", "", []string{"Set 1 / Template 1", "Set 2 / Template 2"}, "Set 1 / Template 1"),
		testutil.NewMultiSelectPrompt("You have not specified an environment. Select a scope or leave empty to update an unscoped variable", "", []string{"Staging", "Production"}, []string{"Staging"}),
		testutil.NewInputPrompt("Value", "", "var value"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantCommonVariables = func(tenant *tenants.Tenant, includeMissingVariables bool) (*variables.GetTenantCommonVariablesResponse, error) {
		return &variables.GetTenantCommonVariablesResponse{
			TenantID: "Tenants-1",
			CommonVariables: []variables.TenantCommonVariable{
				createCommonVariable("TenantVariables-1", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1"}, false),
				createCommonVariable("TenantVariables-2", "LibraryVariableSets-1", "Set 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-2"}, false),
				createCommonVariable("TenantVariables-3", "LibraryVariableSets-2", "Set 2", "Templates-2", "Template 2", "existing value 3", []string{"Environments-1", "Environments-2"}, false),
			},
		}, nil
	}

	opts.GetAllEnvironmentsCallback = func() ([]*environments.Environment, error) {
		staging := environments.NewEnvironment("Staging")
		staging.ID = "Environments-1"
		production := environments.NewEnvironment("Production")
		production.ID = "Environments-2"
		return []*environments.Environment{staging, production}, nil
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
	assert.Equal(t, "Set 1", flags.LibraryVariableSet.Value)
	assert.Empty(t, flags.Project.Value)
	assert.Equal(t, "Template 1", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func TestPromptMissing_ProjectVariable_NoFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Tenant. Please select one:", "", []string{"tenant name", "tenant name 2"}, "tenant name"),
		testutil.NewSelectPrompt("Which type of variable do you want to update?", "", []string{"Library/Common", "Project"}, "Project"),
		testutil.NewSelectPrompt("You have not specified a variable", "", []string{"Project 1 / Template 1", "Project 2 / Template 2"}, "Project 1 / Template 1"),
		testutil.NewMultiSelectPrompt("You have not specified an environment. Select a scope or leave empty to update an unscoped variable", "", []string{"Staging", "Production"}, []string{"Staging"}),
		testutil.NewInputPrompt("Value", "", "var value"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetTenantProjectVariables = func(tenant *tenants.Tenant, includeMissingVariables bool) (*variables.GetTenantProjectVariablesResponse, error) {
		return &variables.GetTenantProjectVariablesResponse{
			TenantID: "Tenants-1",
			ProjectVariables: []variables.TenantProjectVariable{
				createProjectVariable("TenantVariables-1", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 1", []string{"Environments-1"}, false),
				createProjectVariable("TenantVariables-2", "Projects-1", "Project 1", "Templates-1", "Template 1", "existing value 2", []string{"Environments-2"}, false),
				createProjectVariable("TenantVariables-3", "Projects-2", "Project 2", "Templates-2", "Template 2", "existing value 3", []string{"Environments-1", "Environments-2"}, false),
			},
		}, nil
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
	assert.Equal(t, "Project 1", flags.Project.Value)
	assert.Equal(t, "Template 1", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func TestPromptMissingV1_ProjectVariable_AllFlagsProvided(t *testing.T) {
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
		projectTemplate := createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1", false)
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

	err := update.PromptMissingV1(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissingV1_LibraryVariable_AllFlagsProvided(t *testing.T) {
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
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1", false))
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

	err := update.PromptMissingV1(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissingV1_LibraryVariable_NoFlagsProvided(t *testing.T) {
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
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name", resources.ControlTypeSingleLineText, "templateId-1", false))
		libraryVariables.Templates = append(libraryVariables.Templates, createTemplate("var name 2", resources.ControlTypeSingleLineText, "templateId-2", false))
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

	err := update.PromptMissingV1(opts)
	checkRemainingPrompts()
	assert.Equal(t, "var value", flags.Value.Value)
	assert.Equal(t, "lvs", flags.LibraryVariableSet.Value)
	assert.Empty(t, flags.Project.Value)
	assert.Equal(t, "var name", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func TestPromptMissingV1_ProjectVariable_NoFlagsProvided(t *testing.T) {
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
		project1Vars.Templates = append(project1Vars.Templates, createTemplate("project 1 var", resources.ControlTypeSingleLineText, "templateId-1", false))
		project1Vars.Variables["Environments-1"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-1"]["templateId-1"] = core.NewPropertyValue("", false)
		project1Vars.Variables["Environments-2"] = make(map[string]core.PropertyValue)
		project1Vars.Variables["Environments-2"]["templateId-1"] = core.NewPropertyValue("", false)
		project2Vars := variables.ProjectVariable{
			ProjectID:   "Projects-2",
			ProjectName: "Project 2",
			Variables:   make(map[string]map[string]core.PropertyValue),
		}
		project2Vars.Templates = append(project2Vars.Templates, createTemplate("project 2 var", resources.ControlTypeSingleLineText, "templateId-2", false))
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

	err := update.PromptMissingV1(opts)
	checkRemainingPrompts()
	assert.Equal(t, "var value", flags.Value.Value)
	assert.Empty(t, flags.LibraryVariableSet.Value)
	assert.Equal(t, "Project 2", flags.Project.Value)
	assert.Equal(t, "project 2 var", flags.Name.Value)
	assert.Equal(t, "tenant name", flags.Tenant.Value)
	assert.NoError(t, err)
}

func createTemplate(name string, controlType resources.ControlType, templateId string, hasDefault bool) *actiontemplates.ActionTemplateParameter {
	template := actiontemplates.NewActionTemplateParameter()
	template.ID = templateId
	template.Name = name
	template.DisplaySettings = make(map[string]string)
	template.DisplaySettings["Octopus.ControlType"] = string(controlType)

	if hasDefault {
		template.DefaultValue = &core.PropertyValue{Value: "default"}
	}

	return template
}

func createProjectVariable(id string, projectID string, projectName string, templateID string, templateName string, value string, scope []string, hasDefault bool) variables.TenantProjectVariable {
	return variables.TenantProjectVariable{
		Resource:    resources.Resource{ID: id},
		ProjectID:   projectID,
		ProjectName: projectName,
		TemplateID:  templateID,
		Template:    *createTemplate(templateName, resources.ControlTypeSingleLineText, templateID, hasDefault),
		Value:       core.PropertyValue{Value: value},
		Scope:       variables.TenantVariableScope{EnvironmentIds: scope},
	}
}

func createCommonVariable(id string, libraryVariableSetId string, libraryVariableSetName string, templateID string, templateName string, value string, scope []string, hasDefault bool) variables.TenantCommonVariable {
	return variables.TenantCommonVariable{
		Resource:               resources.Resource{ID: id},
		LibraryVariableSetId:   libraryVariableSetId,
		LibraryVariableSetName: libraryVariableSetName,
		TemplateID:             templateID,
		Template:               *createTemplate(templateName, resources.ControlTypeSingleLineText, templateID, hasDefault),
		Value:                  core.PropertyValue{Value: value},
		Scope:                  variables.TenantVariableScope{EnvironmentIds: scope},
	}
}
