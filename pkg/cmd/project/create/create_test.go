package create_test

import (
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectCreate "github.com/OctopusDeploy/cli/pkg/cmd/project/create"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("https://serverurl")
var spinner = &testutil.FakeSpinner{}
var rootResource = testutil.NewRootResource()

func TestAskProjectGroup_WithProvidedName(t *testing.T) {

	value, _, err := projectCreate.AskProjectGroups(nil, "FooBar", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "FooBar", value)
}

func TestAskProjectGroup_WithExistingProjectGroup(t *testing.T) {
	pa := []*testutil.PA{
		{
			Prompt: &survey.Confirm{
				Message: "Would you like to create a new Project Group?",
			},
			Answer: false,
		},
		{
			Prompt: &survey.Select{
				Message: "You have not specified a Project group for this project. Please select one:",
				Options: []string{
					"foo",
					"bar",
				},
			},
			Answer: "bar",
		},
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	getFakeProjectGroups := func() ([]*projectgroups.ProjectGroup, error) {
		return []*projectgroups.ProjectGroup{
			projectgroups.NewProjectGroup("foo"),
			projectgroups.NewProjectGroup("bar"),
		}, nil
	}

	value, _, err := projectCreate.AskProjectGroups(asker, "", getFakeProjectGroups, nil)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "bar", value)
}

func TestAskProjectGroup_WithNewProjectGroup(t *testing.T) {
	pa := []*testutil.PA{
		{
			Prompt: &survey.Confirm{
				Message: "Would you like to create a new Project Group?",
			},
			Answer: true,
		},
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	projectGroupCreateOpts := projectGroupCreate.NewCreateOptions(nil, nil)
	createProjectGroup := func() (string, cmd.Dependable, error) {
		return "foo", projectGroupCreateOpts, nil
	}

	value, pgOpts, err := projectCreate.AskProjectGroups(asker, "", nil, createProjectGroup)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "foo", value)
	assert.Equal(t, projectGroupCreateOpts, pgOpts)
}

func TestPromptForConfigAsCode_NotUsingCac(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("Would you like to use Config as Code?", "", false),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := projectCreate.NewCreateFlags()
	flags.ConfigAsCode.Value = false
	opts := projectCreate.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	err := projectCreate.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.False(t, opts.ConfigAsCode.Value)
}

func TestPromptForConfigAsCode_UsingCacWithProjectStorage(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("Would you like to use Config as Code?", "", true),
		testutil.NewSelectPrompt("Select where to store the Git credentials", "", []string{"Project", "Library"}, "Project"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store configuration.", "https://github.com/blah.git"),
		testutil.NewInputPrompt("Git repository base path", "The path in the repository where Config As Code settings are stored. Default value: '.octopus/'", "./octopus/project"),
		testutil.NewInputPrompt("Git branch", "The default branch to use. Default value: 'main'", "main"),
		testutil.NewInputPrompt("Initial Git commit message", "The commit message used in initializing. Default value: 'Initial commit of deployment process'", "init message"),
		testutil.NewInputPrompt("Git username", "The Git username.", "user1"),
		testutil.NewPasswordPrompt("Git password", "The Git password.", "password"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := projectCreate.NewCreateFlags()
	flags.ConfigAsCode.Value = false
	opts := projectCreate.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	err := projectCreate.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.True(t, opts.ConfigAsCode.Value)
	assert.Equal(t, "project", opts.GitStorage.Value)
	assert.Equal(t, "https://github.com/blah.git", opts.GitUrl.Value)
	assert.Equal(t, "./octopus/project", opts.GitBasePath.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
	assert.Equal(t, "init message", opts.GitInitialCommitMessage.Value)
	assert.Equal(t, "user1", opts.GitUsername.Value)
	assert.Equal(t, "password", opts.GitPassword.Value)
}
