package create_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectCreate "github.com/OctopusDeploy/cli/pkg/cmd/project/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPromptForConfigAsCode_NotUsingCac(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("Would you like to use Config as Code?", "", false),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := projectCreate.NewCreateFlags()
	flags.ConfigAsCode.Value = false

	var convertCallbackCalled bool
	opts := projectCreate.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.ConvertProjectCallback = func() (cmd.Dependable, error) {
		convertCallbackCalled = true
		return nil, nil
	}

	_, err := projectCreate.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.False(t, convertCallbackCalled)
	assert.False(t, opts.ConfigAsCode.Value)
}
