package update_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/variables/update"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_AllFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{}

	variable := variables.NewVariable("")
	variable.ID = "123abc"

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Id.Value = "123"
	flags.Value.Value = "new value"
	flags.EnvironmentsScopes.Value = []string{"test"}
	flags.Project.Value = "make all the things great again"
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{variable},
		}, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}

	opts.GetVariableById = func(ownerId, variableId string) (*variables.Variable, error) {
		return variable, nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_NoFlags_LeaveScope(t *testing.T) {
	project1 := projects.NewProject("Project 1", "Lifecycles-1", "ProjectGroups-1")
	project2 := projects.NewProject("Project 2", "Lifecycles-1", "ProjectGroups-1")

	variable := variables.NewVariable("var1")
	variable.ID = "123abc"
	variable.Type = "String"

	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Project. Please select one:", "", []string{project1.Name, project2.Name}, project1.Name),
		testutil.NewConfirmPromptWithDefault("Do you want to update the variable value?", "", true, false),
		testutil.NewInputPrompt("Value", "", "updated value"),
		testutil.NewSelectPrompt("Do you want to change the variable scoping?", "", []string{"Leave", "Replace", "Unscope"}, "Leave"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{variable},
		}, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) { return project1, nil }
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}
	opts.GetVariableById = func(ownerId, variableId string) (*variables.Variable, error) {
		return variable, nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "updated value", opts.Value.Value)
	assert.Equal(t, "123abc", opts.Id.Value)
}

func TestPromptMissing_NoFlags_ReplaceScope(t *testing.T) {
	project := projects.NewProject("Project 1", "Lifecycles-1", "ProjectGroups-1")

	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you want to update the variable value?", "", false, false),
		testutil.NewSelectPrompt("Do you want to change the variable scoping?", "", []string{"Leave", "Replace", "Unscope"}, "Replace"),
		testutil.NewMultiSelectPrompt("Environment scope", "", []string{"test"}, []string{"test"}),
		testutil.NewMultiSelectPrompt("Process scope", "", []string{"Run, book, run"}, []string{"Run, book, run"}),
		testutil.NewMultiSelectPrompt("Channel scope", "", []string{"Default channel"}, []string{"Default channel"}),
		testutil.NewMultiSelectPrompt("Target scope", "", []string{"Deployment target"}, []string{"Deployment target"}),
		testutil.NewMultiSelectPrompt("Role scope", "", []string{"Role 1"}, []string{"Role 1"}),
		testutil.NewMultiSelectPrompt("Tag scope", "", []string{"tag set/tag 1"}, []string{"tag set/tag 1"}),
		testutil.NewMultiSelectPrompt("Step scope", "", []string{"Step name"}, []string{"Step name"}),
	}

	variable := variables.NewVariable("")
	variable.ID = "123abc"

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Project.Value = project.GetName()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
				Actions:      []*resources.ReferenceDataItem{{ID: "actionId", Name: "Step name"}},
				Channels:     []*resources.ReferenceDataItem{{ID: "Channels-1", Name: "Default channel"}},
				Machines:     []*resources.ReferenceDataItem{{ID: "Machines-1", Name: "Deployment target"}},
				TenantTags:   []*resources.ReferenceDataItem{{ID: "tag set/tag 1", Name: "tag 1"}},
				Roles:        []*resources.ReferenceDataItem{{ID: "Role 1", Name: "Role 1"}},
				Processes:    []*resources.ProcessReferenceDataItem{{ID: "Runbooks-1", Name: "Run, book, run"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{variable},
		}, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) { return project, nil }
	opts.GetVariableById = func(ownerId, variableId string) (*variables.Variable, error) {
		return variable, nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, []string{"test"}, opts.EnvironmentsScopes.Value)
	assert.Equal(t, []string{"Step name"}, opts.StepScopes.Value)
	assert.Equal(t, []string{"Default channel"}, opts.ChannelScopes.Value)
	assert.Equal(t, []string{"Deployment target"}, opts.TargetScopes.Value)
	assert.Equal(t, []string{"tag set/tag 1"}, opts.TagScopes.Value)
	assert.Equal(t, []string{"Role 1"}, opts.RoleScopes.Value)
	assert.Equal(t, []string{"Run, book, run"}, opts.ProcessScopes.Value)
	assert.Equal(t, "123abc", opts.Id.Value)
}

func TestPromptMissing_Unscope(t *testing.T) {
	project := projects.NewProject("Project 1", "Lifecycles-1", "ProjectGroups-1")

	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you want to update the variable value?", "", false, false),
		testutil.NewSelectPrompt("Do you want to change the variable scoping?", "", []string{"Leave", "Replace", "Unscope"}, "Unscope"),
	}

	variable := variables.NewVariable("")
	variable.ID = "123abc"

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Project.Value = project.GetName()
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
				Actions:      []*resources.ReferenceDataItem{{ID: "actionId", Name: "Step name"}},
				Channels:     []*resources.ReferenceDataItem{{ID: "Channels-1", Name: "Default channel"}},
				Machines:     []*resources.ReferenceDataItem{{ID: "Machines-1", Name: "Deployment target"}},
				TenantTags:   []*resources.ReferenceDataItem{{ID: "TenantTags-1", Name: "tag set/tag 1"}},
				Roles:        []*resources.ReferenceDataItem{{ID: "Role 1", Name: "Role 1"}},
				Processes:    []*resources.ProcessReferenceDataItem{{ID: "Runbooks-1", Name: "Run, book, run"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{variable},
		}, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) { return project, nil }
	opts.GetVariableById = func(ownerId, variableId string) (*variables.Variable, error) {
		return variable, nil
	}

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Empty(t, opts.EnvironmentsScopes.Value)
	assert.Empty(t, opts.StepScopes.Value)
	assert.Empty(t, opts.ChannelScopes.Value)
	assert.Empty(t, opts.TargetScopes.Value)
	assert.Empty(t, opts.TagScopes.Value)
	assert.Empty(t, opts.RoleScopes.Value)
	assert.Empty(t, opts.ProcessScopes.Value)
	assert.True(t, opts.Unscoped.Value)
	assert.Equal(t, "123abc", opts.Id.Value)
}

func TestPromptMissing_VersionControlledProject_NoGitRefSupplied(t *testing.T) {
	project := projects.NewProject("Project 1", "Lifecycles-1", "ProjectGroups-1")
	project.IsVersionControlled = true

	pa := []*testutil.PA{
		testutil.NewInputPrompt("GitRef", "The GitRef where the variable is stored", "refs/heads/main"),
		testutil.NewSelectPrompt("Do you want to change the variable scoping?", "", []string{"Leave", "Replace", "Unscope"}, "Unscope"),
	}

	variable := variables.NewVariable("")
	variable.ID = "123abc"

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := update.NewUpdateFlags()
	flags.Project.Value = project.GetName()
	flags.Value.Value = "updated value"
	opts := update.NewUpdateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.Space = spaces.NewSpace("Default")
	opts.Space.ID = "Spaces-1"

	opts.GetProjectVariablesByGitRef = func(spaceId, projectId, gitRef string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID:   "Projects-1",
			SpaceID:   opts.Space.GetID(),
			Variables: []*variables.Variable{variable},
		}, nil
	}

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) { return project, nil }

	err := update.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Empty(t, opts.EnvironmentsScopes.Value)
	assert.Empty(t, opts.StepScopes.Value)
	assert.Empty(t, opts.ChannelScopes.Value)
	assert.Empty(t, opts.TargetScopes.Value)
	assert.Empty(t, opts.TagScopes.Value)
	assert.Empty(t, opts.RoleScopes.Value)
	assert.Empty(t, opts.ProcessScopes.Value)
	assert.True(t, opts.Unscoped.Value)
	assert.Equal(t, "123abc", opts.Id.Value)
}
