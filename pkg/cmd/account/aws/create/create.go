package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an aws account",
		Long:  "Creates an aws account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account aws create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			return createRun(f.Ask, client, cmd.OutOrStdout(), f.Spinner())
		},
	}

	return cmd
}

func createRun(ask question.Asker, client *client.Client, out io.Writer, s *spinner.Spinner) error {
	info, err := question.AskNameAndDescription(ask, "account")
	if err != nil {
		return err
	}
	return CreateAWSAccount(ask, info, client, out, s)
}

func CreateAWSAccount(ask question.Asker, info *question.NameAndDescription, client *client.Client, out io.Writer, s *spinner.Spinner) error {
	var accessKey string
	ask(&survey.Input{
		Message: "Access Key",
		Help:    "The AWS access key to use when authenticating against Amazon Web Services.",
	}, &accessKey, survey.WithValidator(survey.ComposeValidators(
		survey.Required,
	)))

	var secretKey string
	ask(&survey.Password{
		Message: "Secret Key",
		Help:    "The AWS secret key to use when authenticating against Amazon Web Services.",
	}, &secretKey, survey.WithValidator(survey.ComposeValidators(
		survey.Required,
	)))

	awsAccount, err := accounts.NewAmazonWebServicesAccount(info.Name, accessKey, core.NewSensitiveValue(secretKey))
	if err != nil {
		return err
	}
	awsAccount.Description = info.Description

	environmentIDs, err := selectors.EnvironmentsMultiSelect(ask, client, s,
		"Choose the environments that are allowed to use this account.\n"+
			output.Dim("If nothing is selected, the account can be used for deployments to any environment."))
	if err != nil {
		return err
	}
	awsAccount.EnvironmentIDs = environmentIDs

	s.Start()
	createdAccount, err := client.Accounts.Add(awsAccount)
	if err != nil {
		s.Stop()
		return err
	}
	s.Stop()

	_, err = fmt.Fprintf(out, "Successfully created AWS Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		return err
	}
	return nil
}
