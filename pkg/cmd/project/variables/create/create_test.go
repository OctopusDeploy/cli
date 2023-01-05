package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/variables/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_AllFlagsProvided(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Is this a prompted variable?", "", false, false),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "What is a name"
	flags.Description.Value = "The un-describable"
	flags.Value.Value = "new value"
	flags.Type.Value = "Text"
	flags.IsPrompted.Value = false
	flags.EnvironmentsScopes.Value = []string{"test"}
	flags.Project.Value = "Project"
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{},
		}, nil
	}

	err := create.PromptMissing(opts)

	assert.NoError(t, err)
	checkRemainingPrompts()
}

func TestPromptMissing_AllFlagsProvided_PromptedVariable(t *testing.T) {
	pa := []*testutil.PA{}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "What is a name"
	flags.Description.Value = "The un-describable"
	flags.Value.Value = "new value"
	flags.Type.Value = "Text"
	flags.IsPrompted.Value = true
	flags.PromptRequired.Value = true
	flags.PromptDescription.Value = "prompted description"
	flags.PromptType.Value = "String"
	flags.PromptLabel.Value = "prompt?"
	flags.EnvironmentsScopes.Value = []string{"test"}
	flags.Project.Value = "Project"
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{},
		}, nil
	}

	err := create.PromptMissing(opts)

	assert.NoError(t, err)
	checkRemainingPrompts()
}

func TestPromptMissing_NoFlags(t *testing.T) {
	project1 := projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1")
	project2 := projects.NewProject("Project 2", "Lifecycles-1", "ProjectGroups-1")

	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Project. Please select one:", "", []string{project1.Name, project2.Name}, project1.Name),
		testutil.NewInputPrompt("Name", "A name for this variable.", "Ship name"),
		testutil.NewInputPrompt("Description", "A short, memorable, description for this Variable.", "the ship will need a valid name to be able to travel in interstellar space"),
		testutil.NewSelectPrompt("Select the type of the variable", "", []string{"Text", "Sensitive", "Certificate", "Worker Pool", "Azure Account", "Aws Account", "Google Account"}, "Text"),
		testutil.NewConfirmPromptWithDefault("Is this a prompted variable?", "", true, false),
		testutil.NewInputPrompt("Prompt Label", "", "prompt label"),
		testutil.NewInputPrompt("Prompt Description", "A short, memorable, description for this Prompted Variable.", "prompt description"),
		testutil.NewSelectPrompt("Select the control type of the prompted variable", "", []string{"Single line text", "Multi line text", "Checkbox", "Drop down"}, "Single line text"),
		testutil.NewConfirmPromptWithDefault("Is this the prompted variable required to have a value supplied?", "", true, false),
		testutil.NewInputPrompt("Value", "", "Spaceball 1"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return project1, nil
	}
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID:     "Projects-1",
			ScopeValues: &variables.VariableScopeValues{},
			SpaceID:     "Spaces-1",
			Variables:   []*variables.Variable{},
		}, nil
	}

	err := create.PromptMissing(opts)
	assert.NoError(t, err)
	checkRemainingPrompts()
	assert.Equal(t, "Ship name", opts.Name.Value)
	assert.Equal(t, "the ship will need a valid name to be able to travel in interstellar space", opts.Description.Value)
	assert.Equal(t, create.TypeText, opts.Type.Value)
	assert.Equal(t, "Spaceball 1", opts.Value.Value)
	assert.Equal(t, true, opts.IsPrompted.Value)
	assert.Equal(t, "prompt label", opts.PromptLabel.Value)
	assert.Equal(t, "prompt description", opts.PromptDescription.Value)
	assert.Equal(t, create.PromptTypeText, opts.PromptType.Value)
	assert.Equal(t, true, opts.PromptRequired.Value)

}

func TestPromptMissing_PromptedVariableForSelectOptions(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Enter a selection option value (enter blank to end)", "", "value1"),
		testutil.NewInputPrompt("Enter a selection option description", "", "display 1"),
		testutil.NewInputPrompt("Enter a selection option value (enter blank to end)", "", "value2"),
		testutil.NewInputPrompt("Enter a selection option description", "", "display 2"),
		testutil.NewInputPrompt("Enter a selection option value (enter blank to end)", "", ""),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "What is a name"
	flags.Description.Value = "The un-describable"
	flags.Value.Value = "new value"
	flags.Type.Value = "Text"
	flags.IsPrompted.Value = true
	flags.PromptRequired.Value = true
	flags.PromptDescription.Value = "prompted description"
	flags.PromptType.Value = create.PromptTypeDropdown
	flags.PromptLabel.Value = "prompt?"
	flags.EnvironmentsScopes.Value = []string{"test"}
	flags.Project.Value = "Project"
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}
	opts.GetProjectVariables = func(projectId string) (*variables.VariableSet, error) {
		return &variables.VariableSet{
			OwnerID: "Projects-1",
			ScopeValues: &variables.VariableScopeValues{
				Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			},
			SpaceID:   "Spaces-1",
			Variables: []*variables.Variable{},
		}, nil
	}

	err := create.PromptMissing(opts)

	assert.NoError(t, err)
	checkRemainingPrompts()
	assert.Equal(t, []string{"value1|display 1", "value2|display 2"}, opts.PromptSelectOptions.Value)
}
