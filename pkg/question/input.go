package question

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const FlagConfirm = "confirm"

type DeleteFlags struct {
	Confirm *flag.Flag[bool]
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		Confirm: flag.New[bool](FlagConfirm, false),
	}
}

func RegisterDeleteFlag(cmd *cobra.Command, value *bool, resourceDescription string) {
	cmd.Flags().BoolVarP(value, FlagConfirm, "y", false, fmt.Sprintf("Don't ask for confirmation before deleting the %s.", resourceDescription))
}

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

func AskName(ask Asker, messagePrefix string, resourceDescription string, value *string) error {
	if *value == "" {
		if err := ask(&survey.Input{
			Message: messagePrefix + "Name",
			Help:    fmt.Sprintf("A short, memorable, unique name for this %s.", resourceDescription),
		}, value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}
	return nil
}
