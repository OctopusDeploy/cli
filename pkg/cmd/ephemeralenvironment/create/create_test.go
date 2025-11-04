package create_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := create.NewCreateFlags()
	flags.Name.Value = "Hello Ephemeral Environment"
	flags.Project.Value = "Hello Project"

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		// GetAllSpacesCallback: func() ([]*spaces.Space, error) {
		// 	return []*spaces.Space{
		// 		spaces.NewSpace("Explored space")}, nil
		// },
	}

	// Verify that no unexpected prompts were triggered
	create.PromptMissing(opts)
	checkRemainingPrompts()
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this ephemeral environment.", "Hello Ephemeral Environment"),
		testutil.NewInputPrompt("Project Name", "The name of the project to associate the ephemeral environment with.", "Hello Project"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := create.NewCreateFlags()

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
	}

	create.PromptMissing(opts)

	// Verify that all expected prompts were called
	checkRemainingPrompts()
	assert.Equal(t, "Hello Ephemeral Environment", flags.Name.Value)
	assert.Equal(t, "Hello Project", flags.Project.Value)
}
