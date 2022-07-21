package question

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"strings"
)

// If the confirmation was successful, returns a nil error, else returns an error,
// which may be a legitimate error from Survey, or a "Canceled" error if the user
// did not type the correct confirmation string
func AskForDeleteConfirmation(itemType string, itemName string, itemID string) error {
	confirmQuestion := &survey.Question{
		Name: "Confirm Delete",
		Prompt: &survey.Input{
			Message: fmt.Sprintf(`You are about to delete the %s "%s" %s. This action cannot be reversed. To confirm, type the %s name:`,
				itemType, itemName, output.Dimf("(%s)", itemID), itemType),
		},
	}

	var enteredName string
	if err := survey.Ask([]*survey.Question{confirmQuestion}, &enteredName); err != nil {
		return err
	}
	if enteredName != strings.TrimSpace(itemName) {
		// user aborted
		return errors.New("Canceled")
	}
	// confirm yes!
	return nil
}
