package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/branch/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_AllFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Project.Value = "Project"
	flags.BaseBranch.Value = "refs/heads/main"
	flags.Name.Value = "newvar"
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetProjectCallback = func(identifier string) (*projects.Project, error) {
		return projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1"), nil
	}

	err := create.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_NoFlagsSupplied(t *testing.T) {
	project1 := projects.NewProject("Project", "Lifecycles-1", "ProjectGroups-1")
	project1.IsVersionControlled = true
	project2 := projects.NewProject("Project 2", "Lifecycles-1", "ProjectGroups-1")
	project2.IsVersionControlled = true

	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Project. Please select one:", "", []string{project1.Name, project2.Name}, project1.Name),
		testutil.NewInputPrompt("Name", "A name for the new branch.", "newbranch"),
		testutil.NewSelectPrompt("You have not specified a base branch. Please select one:", "", []string{"refs/heads/main", "refs/heads/second-branch"}, "refs/heads/main"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}
	opts.GetAllBranchesCallback = func(projectId string) ([]*projects.GitReference, error) {
		return []*projects.GitReference{
			projects.NewGitBranchReference("main", "refs/heads/main"),
			projects.NewGitBranchReference("second-branch", "refs/heads/second-branch"),
		}, nil
	}

	err := create.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "newbranch", opts.Name.Value)
	assert.Equal(t, "refs/heads/main", opts.BaseBranch.Value)
}
