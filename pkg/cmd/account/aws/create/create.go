package create

import (
	"fmt"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/helper"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	Writer  io.Writer
	Octopus *client.Client
	Ask     question.Asker

	Name         string
	Description  string
	AccessKey    string
	SecretKey    string
	Environments []string

	NoPrompt bool
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask: f.Ask,
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
				if err := validation.IsExistingFile(descriptionFilePath); err != nil {
					return err
				}
				data, err := os.ReadFile(descriptionFilePath)
				if err != nil {
					return err
				}
				opts.Description = string(data)
			}
			opts.NoPrompt = !f.IsPromptEnabled()
			if opts.Environments != nil {
				opts.Environments, err = helper.ResolveEnvironmentNames(opts.Environments, opts.Octopus)
				if err != nil {
					return err
				}
			}
			return CreateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "A short, memorable, unique name for this account.")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "A summary explaining the use of the account to other users.")
	cmd.Flags().StringVar(&opts.AccessKey, "access-key", "", "The AWS access key to use when authenticating against Amazon Web Services.")
	cmd.Flags().StringVar(&opts.SecretKey, "secret-key", "", "The AWS secret key to use when authenticating against Amazon Web Services.")
	cmd.Flags().StringArrayVarP(&opts.Environments, "environments", "e", nil, "The environments that are allowed to use this account")
	cmd.Flags().StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := promptMissing(opts); err != nil {
			return err
		}
	}
	awsAccount, err := accounts.NewAmazonWebServicesAccount(opts.Name, opts.AccessKey, core.NewSensitiveValue(opts.SecretKey))
	if err != nil {
		return err
	}
	awsAccount.Description = opts.Description
	awsAccount.EnvironmentIDs = opts.Environments

	createdAccount, err := opts.Octopus.Accounts.Add(awsAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Writer, "Successfully created AWS Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		return err
	}
	return nil
}

func promptMissing(opts *CreateOptions) error {
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
		if err := opts.Ask(&survey.Input{
			Message: "Access Key",
			Help:    "The AWS access key to use when authenticating against Amazon Web Services.",
		}, &opts.AccessKey, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.SecretKey == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Secret Key",
			Help:    "The AWS secret key to use when authenticating against Amazon Web Services.",
		}, &opts.SecretKey, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Environments == nil {
		environmentIDs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.Octopus,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."))
		if err != nil {
			return err
		}
		opts.Environments = environmentIDs
	}
	return nil
}
