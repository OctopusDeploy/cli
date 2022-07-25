package question

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
)

func DeleteWithConfirmation(ask Asker, itemType string, itemName string, itemID string, doDelete func() error) error {
	var enteredName string
	if err := ask(&survey.Input{
		Message: fmt.Sprintf(
			`You are about to delete the %s "%s" %s. This action cannot be reversed. To confirm, type the %s name:`,
			itemType, itemName, output.Dimf("(%s)", itemID), itemType),
	}, &enteredName); err != nil {
		return err
	}

	if enteredName != itemName {
		return fmt.Errorf("input value %s does match expected value %s", enteredName, itemName)
	}

	if err := doDelete(); err != nil {
		return err
	}

	fmt.Printf("%s The %s, \"%s\" %s was deleted successfully.\n", output.Red("✔"), itemType, itemName, output.Dimf("(%s)", itemID))
	return nil
}
