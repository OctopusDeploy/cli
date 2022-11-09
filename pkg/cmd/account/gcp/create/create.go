package create

import (
	b64 "encoding/base64"
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
	KeyFilePath  *flag.Flag[string]
	Environments *flag.Flag[[]string]
}

type CreateOptions struct {
	*CreateFlags
	Writer  io.Writer
	Octopus *client.Client
	Ask     question.Asker
	Space   string
	Host    string
	CmdPath string

	KeyFileData []byte

	NoPrompt bool
	selectors.GetAllEnvironmentsCallback
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:         flag.New[string]("name", false),
		Description:  flag.New[string]("description", false),
		KeyFilePath:  flag.New[string]("key-file", false),
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
		Short: "Creates a gcp account",
		Long:  "Creates a Google Cloud Account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account gcp create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.GetAllEnvironmentsCallback = func() ([]*environments.Environment, error) {
				return selectors.GetAllEnvironments(*client)
			}
			opts.CmdPath = cmd.CommandPath()
			opts.Octopus = client
			opts.Space = f.GetCurrentSpace().GetID()
			opts.Host = f.GetCurrentHost()
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
	cmd.Flags().StringVarP(&opts.KeyFilePath.Value, opts.KeyFilePath.Name, "K", "", "The json key file to use when authenticating against Google Cloud.")
	cmd.Flags().StringArrayVarP(&opts.Environments.Value, opts.Environments.Name, "e", nil, "The environments that are allowed to use this account")
	cmd.Flags().StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}
	gcpAccount, err := accounts.NewGoogleCloudPlatformAccount(
		opts.Name.Value,
		core.NewSensitiveValue(b64.StdEncoding.EncodeToString(opts.KeyFileData)),
	)
	if err != nil {
		return err
	}
	gcpAccount.Description = opts.Description.Value
	gcpAccount.EnvironmentIDs = opts.Environments.Value

	createdAccount, err := opts.Octopus.Accounts.Add(gcpAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Writer, "Successfully created GCP account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetSlug()))
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/infrastructure/accounts/%s", opts.Host, opts.Space, createdAccount.GetID())
	_, _ = fmt.Fprintf(opts.Writer, "\nView this account on Octopus Deploy: %s\n", link)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.KeyFilePath, opts.Description, opts.Environments)
		_, _ = fmt.Fprintf(opts.Writer, "\nAutomation Command: %s\n", autoCmd)
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

	if len(opts.KeyFileData) == 0 {
		if err := opts.Ask(&survey.Input{
			Message: "Key File Path",
			Help:    "Path to the json key file to use when authenticating against Google Cloud.",
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
