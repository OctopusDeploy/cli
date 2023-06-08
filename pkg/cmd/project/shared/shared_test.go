package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/shared"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAskProjectGroup_WithProvidedName(t *testing.T) {
	value, _, err := shared.AskProjectGroups(nil, "FooBar", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "FooBar", value)
}

func TestAskProjectGroup_WithExistingProjectGroup(t *testing.T) {
	pa := []*testutil.PA{
		{
			Prompt: &surveyext.Select{
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

	value, _, err := shared.AskProjectGroups(asker, "", getFakeProjectGroups, nil)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "bar", value)
}

func TestAskProjectGroup_WithNewProjectGroup(t *testing.T) {
	pa := []*testutil.PA{
		{
			Prompt: &surveyext.Select{
				Message: "You have not specified a Project group for this project. Please select one:",
				Options: []string{
					"foo",
					"bar",
				},
			},
			Answer: constants.PromptCreateNew,
		},
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	getFakeProjectGroups := func() ([]*projectgroups.ProjectGroup, error) {
		return []*projectgroups.ProjectGroup{
			projectgroups.NewProjectGroup("foo"),
			projectgroups.NewProjectGroup("bar"),
		}, nil
	}

	projectGroupCreateOpts := projectGroupCreate.NewCreateOptions(nil, nil)
	createProjectGroup := func() (string, cmd.Dependable, error) {
		return "foo", projectGroupCreateOpts, nil
	}

	value, pgOpts, err := shared.AskProjectGroups(asker, "", getFakeProjectGroups, createProjectGroup)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "foo", value)
	assert.Equal(t, projectGroupCreateOpts, pgOpts)
}
