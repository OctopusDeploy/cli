package create

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/spf13/cobra"
)

type CreateFlags struct {
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	Username     *flag.Flag[string]
	Password     *flag.Flag[string]
	Environments *flag.Flag[[]string]
}

type CreateOptions struct {
	*CreateFlags
	Writer   io.Writer
	Octopus  *client.Client
	Ask      question.Asker
	Space    string
	NoPrompt bool
	CmdPath  string
	Host     string
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:         flag.New[string]("name", false),
		Description:  flag.New[string]("description", false),
		Username:     flag.New[string]("username", false),
		Password:     flag.New[string]("password", true),
		Environments: flag.New[[]string]("environment", false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:         f.Ask,
		CreateFlags: NewCreateFlags(),
	}
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a username/password account",
		Long:  "Creates a Username and Password Account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account username create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, _ []string) error {
			octopus, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.CmdPath = cmd.CommandPath()
			opts.Octopus = octopus
			opts.Host = f.GetCurrentHost()
			opts.Space = f.GetCurrentSpace().GetID()
			opts.Writer = cmd.OutOrStdout()
			if descriptionFilePath != "" {
				if err := validation.IsExistingFile(descriptionFilePath); err != nil {
					return err
				}
				data, err := os.ReadFile(descriptionFilePath)
				if err != nil {
					return err
				}
				opts.Description.Value = string(data)
			}
			opts.NoPrompt = !f.IsPromptEnabled()
			if opts.Environments.Value != nil {
				opts.Environments.Value, err = helper.ResolveEnvironmentNames(opts.Environments.Value, opts.Octopus)
				if err != nil {
					return err
				}
			}
			return CreateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name.Value, opts.Name.Name, "n", "", "A short, memorable, unique name for this account.")
	cmd.Flags().StringVarP(&opts.Description.Value, opts.Description.Name, "d", "", "A summary explaining the use of the account to other users.")
	cmd.Flags().StringVarP(&opts.Username.Value, opts.Username.Name, "u", "", "The username to use when authenticating against the remote host.")
	cmd.Flags().StringVarP(&opts.Password.Value, opts.Password.Name, "p", "", "The password to use to when authenticating against the remote host.")
	cmd.Flags().StringArrayVarP(&opts.Environments.Value, opts.Environments.Name, "e", nil, "The environments that are allowed to use this account.")
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
		opts.Name.Value,
	)
	if err != nil {
		return err
	}
	usernameAccount.Username = opts.Username.Value
	usernameAccount.Password = core.NewSensitiveValue(opts.Password.Value)
	usernameAccount.Description = opts.Description.Value
	usernameAccount.EnvironmentIDs = opts.Environments.Value

	createdAccount, err := opts.Octopus.Accounts.Add(usernameAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Writer, "Successfully created Token Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/infrastructure/accounts/%s", opts.Host, opts.Space, createdAccount.GetID())
	_, _ = fmt.Fprintf(opts.Writer, "\nView this account on Octopus Deploy: %s\n", link)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Username, opts.Password, opts.Description, opts.Environments)
		_, _ = fmt.Fprintf(opts.Writer, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func promptMissing(opts *CreateOptions) error {
	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    "A short, memorable, unique name for this account.",
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Description.Value == "" {
		if err := opts.Ask(&surveyext.OctoEditor{
			Editor: &survey.Editor{
				Message:  "Description",
				Help:     "A summary explaining the use of the account to other users.",
				FileName: "*.md",
			},
			Optional: true,
		}, &opts.Description.Value); err != nil {
			return err
		}
	}

	if opts.Username.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Username",
			Help:    "The username to use when authenticating against the remote host.",
		}, &opts.Username.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Password.Value == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Password",
			Help:    "The password to use to when authenticating against the remote host.",
		}, &opts.Password.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Environments.Value == nil {
		envs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.Octopus,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."), 0)
		if err != nil {
			return err
		}
		opts.Environments.Value = util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })
	}
	return nil
}
