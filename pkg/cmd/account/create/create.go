package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an account in an instance of Octopus Deploy",
		Long:  "Creates an account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createRun(f, cmd.OutOrStdout())
		},
	}

	return cmd
}

func createRun(f factory.Factory, w io.Writer) error {
	octopus, err := f.Client(true)
	if err != nil {
		return err
	}

	existingAccounts, err := octopus.Accounts.GetAll()
	if err != nil {
		return err
	}

	accountNames := []string{}
	for _, existingAccount := range existingAccounts {
		accountNames = append(accountNames, existingAccount.GetName())
	}

	accountTypes := []string{
		"AWS Account",
		"Azure Subscription",
		"Google Cloud Account",
		"SSH Key Pair",
		"Username/Password",
		"Token",
	}

	var accountType string
	err = f.Ask(&survey.Select{
		Help:    "The type of account being created.",
		Message: "Account Type",
		Options: accountTypes,
	}, &accountType)
	if err != nil {
		return err
	}

	var name string
	err = f.Ask(&survey.Input{
		Help:    "The name of the account being created.",
		Message: "Name",
	}, &name, survey.WithValidator(survey.ComposeValidators(
		survey.MaxLength(200),
		survey.MinLength(1),
		survey.Required,
		validation.NotEquals(accountNames, "an account with this name already exists"),
	)))
	if err != nil {
		return err
	}

	var description string
	err = f.Ask(&survey.Input{
		Help:    "A summary explaining the use of the account to other users.",
		Message: "Description",
	}, &description)
	if err != nil {
		return err
	}

	switch accountType {
	case "Azure Subscription":
		createAzureSubscriptionRun(f.Ask, octopus, w)
	}

	// TODO: use the name; create the account

	return nil
}

func createAzureSubscriptionRun(ask question.Asker, octopus *client.Client, w io.Writer) error {
	var subscriptionID string
	err := ask(&survey.Input{
		Help:    "Your Azure subscription ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		Message: "Subscription ID",
	}, &subscriptionID, survey.WithValidator(survey.ComposeValidators(
		survey.Required,
		validation.IsUuid,
	)))
	if err != nil {
		return err
	}

	return nil
}
