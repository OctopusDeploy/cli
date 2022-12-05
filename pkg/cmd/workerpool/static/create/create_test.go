package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/static/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "name"
	flags.Description.Value = "description"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptMissing_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this Dynamic Worker Pool.", "name"),
		testutil.NewInputPrompt("Description", "A short, memorable, description for this Dynamic Worker Pool.", "description"),
		testutil.NewSelectPrompt("Select the worker type to use", "", []string{"Ubuntu (UbuntuDefault)", "Windows (WindowsDefault)"}, "Windows (WindowsDefault)"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)

	assert.Equal(t, "name", flags.Name.Value)
	assert.Equal(t, "description", flags.Description.Value)
}
