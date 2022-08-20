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
	Username     string
	Password     string
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
		Short: "Creates a username/password account",
		Long:  "Creates a Username and Password Account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account username create"
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
	cmd.Flags().StringVarP(&opts.Username, "username", "u", "", "The username to use when authenticating against the remote host.")
	cmd.Flags().StringVarP(&opts.Password, "password", "p", "", "The password to use to when authenticating against the remote host.")
	cmd.Flags().StringArrayVarP(&opts.Environments, "environments", "e", nil, "The environments that are allowed to use this account.")
	cmd.Flags().StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`.")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := promptMissing(opts); err != nil {
			return err
		}
	}
	usernameAccount, err := accounts.NewUsernamePasswordAccount(
		opts.Name,
	)
	if err != nil {
		return err
	}
	usernameAccount.Username = opts.Username
	usernameAccount.Password = core.NewSensitiveValue(opts.Password)
	usernameAccount.Description = opts.Description
	usernameAccount.EnvironmentIDs = opts.Environments

	createdAccount, err := opts.Octopus.Accounts.Add(usernameAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Writer, "Successfully created Token Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
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

	if opts.Username == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Username",
			Help:    "The username to use when authenticating against the remote host.",
		}, &opts.Username, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Password == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Password",
			Help:    "The password to use to when authenticating against the remote host.",
		}, &opts.Password, survey.WithValidator(survey.ComposeValidators(
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
