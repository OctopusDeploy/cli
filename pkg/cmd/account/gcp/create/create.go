package create

import (
	b64 "encoding/base64"
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
	KeyFileData  []byte
	Environments []string

	NoPrompt bool
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:     f.Ask,
		Spinner: f.Spinner(),
	}
	descriptionFilePath := ""
	keyFilePath := ""

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an gcp account",
		Long:  "Creates an Google Cloud Account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account gcp create"
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
			if keyFilePath != "" {
				data, err := os.ReadFile(keyFilePath)
				if err != nil {
					return err
				}
				opts.KeyFileData = data
			}
			opts.NoPrompt = !f.IsPromptEnabled()
			if opts.Environments != nil {
				opts.Environments, err = helper.ResolveEnvironmentNames(opts.Environments, opts.Octopus, opts.Spinner)
				if err != nil {
					return err
				}
			}
			return CreateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "A short, memorable, unique name for this account.")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "A summary explaining the use of the account to other users.")
	cmd.Flags().StringVarP(&keyFilePath, "key-file", "K", "", "The json key file to use when authenticating against Google Cloud.")
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
	gcpAccount, err := accounts.NewGoogleCloudPlatformAccount(
		opts.Name,
		core.NewSensitiveValue(b64.StdEncoding.EncodeToString(opts.KeyFileData)),
	)
	if err != nil {
		return err
	}
	gcpAccount.Description = opts.Description
	gcpAccount.EnvironmentIDs = opts.Environments

	opts.Spinner.Start()
	createdAccount, err := opts.Octopus.Accounts.Add(gcpAccount)
	if err != nil {
		opts.Spinner.Stop()
		return err
	}
	opts.Spinner.Stop()

	_, err = fmt.Fprintf(opts.Writer, "Successfully created GCP Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
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

	if len(opts.KeyFileData) == 0 {
		keyFilePath := ""
		opts.Ask(&survey.Input{
			Message: "Key File Path",
			Help:    "Path to the json key file to use when authenticating against Google Cloud.",
		}, &keyFilePath, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		)))
		data, err := os.ReadFile(keyFilePath)
		if err != nil {
			return err
		}
		opts.KeyFileData = data
	}

	if opts.Environments == nil {
		environmentIDs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.Octopus, opts.Spinner,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."))
		if err != nil {
			return err
		}
		opts.Environments = environmentIDs
	}
	return nil
}
