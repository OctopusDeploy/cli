package question

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
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

type NameAndDescriptionOutput struct {
	Name        string
	Description string
}

func NameAndDescription(ask Asker, itemType string) (*NameAndDescriptionOutput, error) {
	output := &NameAndDescriptionOutput{}
	var name string
	if err := ask(&survey.Input{
		Message: "Name",
		Help:    fmt.Sprintf("The name of the %s being created.", itemType),
	}, &name, survey.WithValidator(survey.ComposeValidators(
		survey.MaxLength(200),
		survey.MinLength(1),
		survey.Required,
	))); err != nil {
		return nil, err
	}
	output.Name = name
	var description string
	if err := ask(&surveyext.OctoEditor{
		Editor: &survey.Editor{
			Message:  "Description",
			Help:     fmt.Sprintf("A summary explaining the use of the %s to other users.", itemType),
			FileName: "*.md",
		},
		Optional: true,
	}, &description); err != nil {
		return nil, err
	}
	output.Description = description
	return output, nil
}
