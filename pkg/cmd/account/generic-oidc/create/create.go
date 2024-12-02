package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/helper"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
	"os"
)

type CreateFlags struct {
	Name                 *flag.Flag[string]
	Description          *flag.Flag[string]
	Environments         *flag.Flag[[]string]
	ExecutionSubjectKeys *flag.Flag[[]string]
	Audience             *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	selectors.GetAllEnvironmentsCallback
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                 flag.New[string]("name", false),
		Description:          flag.New[string]("description", false),
		Environments:         flag.New[[]string]("environment", false),
		ExecutionSubjectKeys: flag.New[[]string]("execution-subject-keys", false),
		Audience:             flag.New[string]("audience", false),
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
		Short:   "Create an Generic OpenID Connect account",
		Long:    "Create an Generic OpenID Connect account in Octopus Deploy",
		Example: heredoc.Docf("$ %s account generic-oidc create", constants.ExecutableName),
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
			opts.NoPrompt = !f.IsPromptEnabled()

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
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Value, "d", "", "A summary explaining the use of the account to other users.")
	flags.StringArrayVarP(&createFlags.Environments.Value, createFlags.Environments.Name, "e", nil, "The environments that are allowed to use this account")
	flags.StringArrayVarP(&createFlags.ExecutionSubjectKeys.Value, createFlags.ExecutionSubjectKeys.Name, "E", nil, "The subject keys used for a deployment or runbook")
	flags.StringVar(&createFlags.Audience.Value, createFlags.Audience.Name, "", "The audience claim for the federated credentials. Defaults to api://default")
	flags.StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}
	var createdAccount accounts.IAccount
	oidcAccount, err := accounts.NewGenericOIDCAccount(
		opts.Name.Value,
	)
	if err != nil {
		return err
	}
	oidcAccount.DeploymentSubjectKeys = opts.ExecutionSubjectKeys.Value
	oidcAccount.Audience = opts.Audience.Value
	oidcAccount.Description = opts.Description.Value

	createdAccount, err = opts.Client.Accounts.Add(oidcAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created Generic account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetSlug()))
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/infrastructure/accounts/%s", opts.Host, opts.Space.GetID(), createdAccount.GetID())
	fmt.Fprintf(opts.Out, "\nView this account on Octopus Deploy: %s\n", link)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath,
			opts.Name,
			opts.Description,
			opts.Environments,
			opts.ExecutionSubjectKeys,
		)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
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

	var err error
	if len(opts.ExecutionSubjectKeys.Value) == 0 {
		opts.ExecutionSubjectKeys.Value, err = promptSubjectKeys(opts.Ask, "Deployment and Runbook subject keys", []string{"space", "environment", "project", "tenant", "runbook", "account", "type"})
		if err != nil {
			return err
		}
	}

	if opts.Audience.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Audience",
			Default: "api://default",
			Help:    "Set this if you need to override the default Audience value.",
		}, &opts.Audience.Value); err != nil {
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

func promptSubjectKeys(ask question.Asker, message string, opts []string) ([]string, error) {
	keys, err := question.MultiSelectMap(ask, message, opts, func(item string) string { return item }, false)
	if err != nil {
		return nil, err
	}
	if len(keys) > 0 {
		return keys, nil
	}

	return nil, nil
}
