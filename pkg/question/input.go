package question

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const FlagConfirm = "confirm"

type ConfirmFlags struct {
	Confirm *flag.Flag[bool]
}

func NewConfirmFlags() *ConfirmFlags {
	return &ConfirmFlags{
		Confirm: flag.New[bool](FlagConfirm, false),
	}
}

func RegisterConfirmDeletionFlag(cmd *cobra.Command, value *bool, resourceDescription string) {
	cmd.Flags().BoolVarP(value, FlagConfirm, "y", false, fmt.Sprintf("Don't ask for confirmation before deleting the %s.", resourceDescription))
}

// describeConfirmationFailure turns the internal "prompt disabled" error into
// something a caller can act on. Deletion asks the user to type the item's name
// to confirm, which cannot happen with prompting turned off, so --no-prompt on
// its own reports only "prompt disabled" and deletes nothing. Point at the flag
// that actually skips the confirmation rather than deleting unasked, since that
// is not something to infer from --no-prompt.
func describeConfirmationFailure(err error, itemType string) error {
	var promptDisabled *cliErrors.PromptDisabledError
	if errors.As(err, &promptDisabled) {
		return fmt.Errorf(
			"cannot confirm deletion of the %s while prompting is disabled; pass --%s to delete it without confirmation",
			itemType, FlagConfirm)
	}

	return err
}

func DeleteWithConfirmation(ask Asker, itemType string, itemName string, itemID string, doDelete func() error) error {
	var enteredName string
	if err := ask(&survey.Input{
		Message: fmt.Sprintf(
			`You are about to delete the %s "%s" %s. This action cannot be reversed. To confirm, type the %s name:`,
			itemType, itemName, output.Dimf("(%s)", itemID), itemType),
	}, &enteredName); err != nil {
		return describeConfirmationFailure(err, itemType)
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

func AskDescription(ask Asker, messagePrefix string, resourceDescription string, value *string) error {
	if *value == "" {
		if err := ask(&survey.Input{
			Message: messagePrefix + "Description",
			Help:    fmt.Sprintf("A short, memorable, description for this %s.", resourceDescription),
		}, value); err != nil {
			return err
		}
	}

	return nil
}
