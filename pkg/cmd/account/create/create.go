package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f apiclient.ClientFactory) *cobra.Command {
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

func createRun(f apiclient.ClientFactory, w io.Writer) error {
	client, err := f.Get(true)
	if err != nil {
		return err
	}

	existingAccounts, err := client.Accounts.GetAll()
	if err != nil {
		return err
	}

	accountNames := []string{}
	for _, existingAccount := range existingAccounts {
		accountNames = append(accountNames, existingAccount.GetName())
	}

	var name string
	err = question.AskOne(&survey.Input{
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

	// TODO: use the name; create the account

	return nil
}
