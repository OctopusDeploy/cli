package create

import (
	b64 "encoding/base64"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"os"

	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/helper"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/spf13/cobra"
)

type CreateFlags struct {
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	KeyFilePath  *flag.Flag[string]
	Username     *flag.Flag[string]
	Passphrase   *flag.Flag[string]
	Environments *flag.Flag[[]string]
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	KeyFileData []byte
	selectors.GetAllEnvironmentsCallback
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:         flag.New[string]("name", false),
		Description:  flag.New[string]("description", false),
		KeyFilePath:  flag.New[string]("private-key", false),
		Username:     flag.New[string]("username", false),
		Passphrase:   flag.New[string]("passphrase", true),
		Environments: flag.New[[]string]("environment", false),
	}
}

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(dependencies.Client)
		},
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a SSH Key Pair account",
		Long:    "Create a SSH Key Pair account in Octopus Deploy",
		Example: heredoc.Docf("$ %s account ssh create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))
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
			if opts.KeyFilePath.Value != "" {
				if err := validation.IsExistingFile(opts.KeyFilePath.Value); err != nil {
					return err
				}
				data, err := os.ReadFile(opts.KeyFilePath.Value)
				if err != nil {
					return err
				}
				opts.KeyFileData = data
			}
			if opts.Environments.Value != nil {
				env, err := helper.ResolveEnvironmentNames(opts.Environments.Value, opts.Client)
				if err != nil {
					return err
				}
				opts.Environments.Value = env
			}
			return CreateRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this account.")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "A summary explaining the use of the account to other users.")
	flags.StringVarP(&createFlags.KeyFilePath.Value, createFlags.KeyFilePath.Name, "K", "", "Path to the private key file portion of the key pair.")
	flags.StringVarP(&createFlags.Username.Value, createFlags.Username.Name, "u", "", "The username to use when authenticating against the remote host.")
	flags.StringVarP(&createFlags.Passphrase.Value, createFlags.Passphrase.Name, "p", "", "The passphrase for the private key, if required.")
	flags.StringArrayVarP(&createFlags.Environments.Value, createFlags.Environments.Name, "e", nil, "The environments that are allowed to use this account.")
	flags.StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`.")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}
	sshAccount, err := accounts.NewSSHKeyAccount(
		opts.Name.Value,
		opts.Username.Value,
		core.NewSensitiveValue(b64.StdEncoding.EncodeToString(opts.KeyFileData)),
	)
	if err != nil {
		return err
	}
	sshAccount.Description = opts.Description.Value
	sshAccount.EnvironmentIDs = opts.Environments.Value
	if opts.Passphrase.Value != "" {
		sshAccount.PrivateKeyPassphrase = core.NewSensitiveValue(opts.Passphrase.Value)
	}

	createdAccount, err := opts.Client.Accounts.Add(sshAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created SSH account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetSlug()))
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/infrastructure/accounts/%s", opts.Host, opts.Space.GetID(), createdAccount.GetID())
	_, _ = fmt.Fprintf(opts.Out, "\nView this account on Octopus Deploy: %s\n", link)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.KeyFilePath, opts.Passphrase, opts.Description, opts.Environments)
		_, _ = fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func PromptMissing(opts *CreateOptions) error {
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

	if len(opts.KeyFileData) == 0 {
		if err := opts.Ask(&survey.Input{
			Message: "Private Key File Path",
			Help:    "Path to the the private key file portion of the key pair.",
		}, &opts.KeyFilePath.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsExistingFile,
		))); err != nil {
			return err
		}
		data, err := os.ReadFile(opts.KeyFilePath.Value)
		if err != nil {
			return err
		}
		opts.KeyFileData = data
	}

	if opts.Passphrase.Value == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Passphrase",
			Help:    "The passphrase for the private key, if required.",
		}, &opts.Passphrase.Value); err != nil {
			return err
		}
	}

	if opts.Environments.Value == nil {
		envs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.GetAllEnvironmentsCallback,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."), false)
		if err != nil {
			return err
		}
		opts.Environments.Value = util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })
	}
	return nil
}
