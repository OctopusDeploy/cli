package create

import (
	"fmt"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	Writer  io.Writer
	Octopus *client.Client
	Ask     question.Asker
	Spinner *spinner.Spinner

	Name         string
	Description  string
	AccessKey    string
	SecretKey    string
	Environments []string
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:     f.Ask,
		Spinner: f.Spinner(),
	}
	descriptionFilePath := ""

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
			opts.Octopus = client
			opts.Writer = cmd.OutOrStdout()
			if descriptionFilePath != "" {
				data, err := os.ReadFile(descriptionFilePath)
				if err != nil {
					return err
				}
				opts.Description = string(data)
			}
			return CreateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "A short, memorable, unique name for this account.")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "A summary explaining the use of the account to other users.")
	cmd.Flags().StringVar(&opts.AccessKey, "access-key", "", "The AWS access key to use when authenticating against Amazon Web Services.")
	cmd.Flags().StringVar(&opts.SecretKey, "secret-key", "", "The AWS secret key to use when authenticating against Amazon Web Services.")
	cmd.Flags().StringArrayVarP(&opts.Environments, "environments", "e", nil, "Choose the environments that are allowed to use this account")
	cmd.Flags().StringVarP(&descriptionFilePath, "description-file", "F", "", "Read the description from `file`")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if opts.Name == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    "A short, memorable, unique name for this account.",
		}, &opts.Name, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Description == "" {
		if err := opts.Ask(&surveyext.OctoEditor{
			Editor: &survey.Editor{
				Message:  "Description",
				Help:     "A summary explaining the use of the account to other users.",
				FileName: "*.md",
			},
			Optional: true,
		}, &opts.Description); err != nil {
			return err
		}
	}

	if opts.AccessKey == "" {
		opts.Ask(&survey.Input{
			Message: "Access Key",
			Help:    "The AWS access key to use when authenticating against Amazon Web Services.",
		}, &opts.AccessKey, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		)))
	}

	if opts.SecretKey == "" {
		opts.Ask(&survey.Password{
			Message: "Secret Key",
			Help:    "The AWS secret key to use when authenticating against Amazon Web Services.",
		}, &opts.SecretKey, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		)))
	}

	awsAccount, err := accounts.NewAmazonWebServicesAccount(opts.Name, opts.AccessKey, core.NewSensitiveValue(opts.SecretKey))
	if err != nil {
		return err
	}
	awsAccount.Description = opts.Description

	if opts.Environments == nil {
		environmentIDs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.Octopus, opts.Spinner,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."))
		if err != nil {
			return err
		}
		opts.Environments = environmentIDs
	}
	awsAccount.EnvironmentIDs = opts.Environments

	opts.Spinner.Start()
	createdAccount, err := opts.Octopus.Accounts.Add(awsAccount)
	if err != nil {
		opts.Spinner.Stop()
		return err
	}
	opts.Spinner.Stop()

	_, err = fmt.Fprintf(opts.Writer, "Successfully created AWS Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		return err
	}
	return nil
}

func createPrompts() {

}
