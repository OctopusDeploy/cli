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
