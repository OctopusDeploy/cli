package question

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
)

type SurveyAsker struct {
}

func (a *SurveyAsker) SimpleTextQuestion(name string, promptMessage string) (string, error) {
	surveyQuestion := &survey.Question{
		Name:   name,
		Prompt: &survey.Input{Message: promptMessage},
	}

	var enteredValue string
	if err := Ask([]*survey.Question{surveyQuestion}, &enteredValue); err != nil {
		return "", err
	}
	return enteredValue, nil
}

type Asker interface {
	SimpleTextQuestion(name string, promptMessage string) (string, error)
}

// If the confirmation was successful, returns a nil error, else returns an error,
// which may be a legitimate error from Survey, or a "Canceled" error if the user
// did not type the correct confirmation string
func AskForDeleteConfirmation(ask Asker, itemType string, itemName string, itemID string, doDelete func() error) error {
	enteredName, err := ask.SimpleTextQuestion(
		"Confirm Delete",
		fmt.Sprintf(`You are about to delete the %s "%s" %s. This action cannot be reversed. To confirm, type the %s name:`,
			itemType, itemName, output.Dimf("(%s)", itemID), itemType))

	if err != nil {
		return err
	}
	if enteredName != itemName {
		// user aborted
		return errors.New("Canceled")
	}

	if err := doDelete(); err != nil {
		return err
	}

	fmt.Printf("%s The %s, \"%s\" %s was deleted successfully.\n", output.Red("✔"), itemType, itemName, output.Dimf("(%s)", itemID))
	return nil
}
