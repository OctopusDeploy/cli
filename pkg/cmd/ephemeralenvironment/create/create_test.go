package create_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/create"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/stretchr/testify/assert"
)

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {

	project1 := fixtures.NewProject("Spaces-1", "Projects-1", "Test1", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	project2 := fixtures.NewProject("Spaces-1", "Projects-2", "Test2", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-2")

	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := create.NewCreateFlags()
	flags.Name.Value = "Hello Ephemeral Environment"
	flags.Project.Value = "Hello Project"

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
	}
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}

	// Check that no unexpected prompts were triggered
	create.PromptMissing(opts)
	checkRemainingPrompts()
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	project1 := fixtures.NewProject("Spaces-1", "Projects-1", "Hello Project 1", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	project2 := fixtures.NewProject("Spaces-1", "Projects-2", "Hello Project 2", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-2")

	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this ephemeral environment.", "Hello Ephemeral Environment"),
		testutil.NewSelectPrompt("Select the project to associate the ephemeral environment with:", "", []string{project1.Name, project2.Name}, project1.Name),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := create.NewCreateFlags()

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		GetAllProjectsCallback: func() ([]*projects.Project, error) {
			return []*projects.Project{project1, project2}, nil
		},
	}

	create.PromptMissing(opts)

	// Check that all expected prompts were called
	checkRemainingPrompts()
	assert.Equal(t, "Hello Ephemeral Environment", flags.Name.Value)
	assert.Equal(t, project1.Name, flags.Project.Value)
}
