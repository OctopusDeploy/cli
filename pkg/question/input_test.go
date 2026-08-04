package question

import (
	"errors"
	"testing"

	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// Deletion asks the user to retype the item's name, which cannot happen with
// prompting disabled. On its own that surfaced only "prompt disabled", with no
// hint that --confirm is the way to delete without being asked.
func TestDescribeConfirmationFailure_ExplainsPromptDisabled(t *testing.T) {
	err := describeConfirmationFailure(&cliErrors.PromptDisabledError{}, "deployment target")

	assert.EqualError(t, err,
		"cannot confirm deletion of the deployment target while prompting is disabled; pass --confirm to delete it without confirmation")
}

func TestDescribeConfirmationFailure_ExplainsWrappedPromptDisabled(t *testing.T) {
	wrapped := errors.Join(errors.New("while deleting"), &cliErrors.PromptDisabledError{})

	err := describeConfirmationFailure(wrapped, "package")

	assert.EqualError(t, err,
		"cannot confirm deletion of the package while prompting is disabled; pass --confirm to delete it without confirmation")
}

func TestDescribeConfirmationFailure_LeavesOtherErrorsAlone(t *testing.T) {
	original := errors.New("interrupted")

	assert.Same(t, original, describeConfirmationFailure(original, "package"))
}

func TestDescribeConfirmationFailure_NoError(t *testing.T) {
	assert.NoError(t, describeConfirmationFailure(nil, "package"))
}
