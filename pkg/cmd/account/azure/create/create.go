package create

import (
	"fmt"
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
	"github.com/briandowns/spinner"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"io"
	"os"
)

type CreateOptions struct {
	Writer  io.Writer
	Octopus *client.Client
	Ask     question.Asker
	Spinner *spinner.Spinner

	Name                   string
	Description            string
	Environments           []string
	SubscriptionID         string
	TenantID               string
	ApplicationID          string
	ApplicationPasswordKey string
	AzureEnvironment       string

	NoPrompt bool
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:     f.Ask,
		Spinner: f.Spinner(),
	}
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an azure account",
		Long:  "Creates an azure account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account azure create"
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

			if opts.SubscriptionID != "" {
				if err := validation.IsUuid(opts.SubscriptionID); err != nil {
					return err
				}
			}
			if opts.TenantID != "" {
				if err := validation.IsUuid(opts.TenantID); err != nil {
					return err
				}
			}
			if opts.ApplicationID != "" {
				if err := validation.IsUuid(opts.ApplicationID); err != nil {
					return err
				}
			}
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
	cmd.Flags().StringVar(&opts.SubscriptionID, "subscription-id", "", "Your Azure subscription ID.")
	cmd.Flags().StringVar(&opts.TenantID, "tenant-id", "", "Your Azure Active Directory Tenant ID.")
	cmd.Flags().StringVar(&opts.ApplicationID, "application-id", "", "Your Azure Active Directory Application ID.")
	cmd.Flags().StringVar(&opts.ApplicationPasswordKey, "application-key", "", "The password for the Azure Active Directory application.")
	cmd.Flags().StringArrayVarP(&opts.Environments, "environments", "e", nil, "The environments that are allowed to use this account")
	cmd.Flags().StringVar(&opts.AzureEnvironment, "azure-environment", "", "Configure isolated Azure Environment.")
	cmd.Flags().StringVarP(&descriptionFilePath, "description-file", "D", "", "Read the description from `file`")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := promptMissing(opts); err != nil {
			return err
		}
	}
	var createdAccount accounts.IAccount
	subId, err := uuid.Parse(opts.SubscriptionID)
	if err != nil {
		return err
	}
	tenantID, err := uuid.Parse(opts.TenantID)
	if err != nil {
		return err
	}
	appID, err := uuid.Parse(opts.ApplicationID)
	servicePrincipalAccount, err := accounts.NewAzureServicePrincipalAccount(
		opts.Name,
		subId,
		tenantID,
		appID,
		core.NewSensitiveValue(opts.ApplicationPasswordKey),
	)
	servicePrincipalAccount.Description = opts.Description
	if err != nil {
		return err
	}
	createdAccount, err = opts.Octopus.Accounts.Add(servicePrincipalAccount)
	if err != nil {
		return err
	}

	opts.Spinner.Start()
	_, err = fmt.Fprintf(opts.Writer, "Successfully created azure Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		opts.Spinner.Stop()
		return err
	}
	opts.Spinner.Stop()
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

	if opts.SubscriptionID == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Subscription ID",
			Help:    "Your Azure subscription ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.SubscriptionID, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.TenantID == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Tenant ID",
			Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.TenantID, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.ApplicationID == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Application ID",
			Help:    "Your Azucire Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.ApplicationID, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.ApplicationPasswordKey == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Application Password / Key",
			Help:    "The password for the Azure Active Directory application. This value is known as Key in the Azure Portal, and Password in the API.",
		}, &opts.ApplicationPasswordKey, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
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
