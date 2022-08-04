package create

import (
	"fmt"
	"io"
	"os"
	"strings"

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
	ADEndpointBaseUrl      string
	RMBaseUri              string

	NoPrompt bool
}

var azureEnvMap = map[string]string{
	"Global Cloud (Default)": "AzureCloud",
	"China Cloud":            "AzureChinaCloud",
	"German Cloud":           "AzureGermanCloud",
	"US Government":          "AzureUSGovernment",
}
var azureADEndpointBaseUri = map[string]string{
	"AzureCloud":        "https://login.microsoftonline.com/",
	"AzureChinaCloud":   "https://login.chinacloudapi.cn/",
	"AzureGermanCloud":  "https://login.microsoftonline.de/",
	"AzureUSGovernment": "https://login.microsoftonline.us/",
}
var azureResourceManagementBaseUri = map[string]string{
	"AzureCloud":        "https://management.azure.com/",
	"AzureChinaCloud":   "https://management.chinacloudapi.cn/",
	"AzureGermanCloud":  "https://management.microsoftazure.de/",
	"AzureUSGovernment": "https://management.usgovcloudapi.net/",
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:     f.Ask,
		Spinner: f.Spinner(),
	}
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an Azure account",
		Long:  "Creates an Azure account in an instance of Octopus Deploy.",
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
			if opts.AzureEnvironment != "" {
				isAzureEnvCorrect := false
				for _, value := range azureEnvMap {
					if strings.EqualFold(value, opts.AzureEnvironment) {
						opts.AzureEnvironment = value
						isAzureEnvCorrect = true
						break
					}
				}
				if !isAzureEnvCorrect {
					return fmt.Errorf("the Azure environment %s is not correct, please use AzureChinaCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernment", opts.AzureEnvironment)
				}
				if opts.RMBaseUri == "" && opts.NoPrompt {
					opts.RMBaseUri = azureResourceManagementBaseUri[opts.AzureEnvironment]
				}
				if opts.ADEndpointBaseUrl == "" && opts.NoPrompt {
					opts.ADEndpointBaseUrl = azureADEndpointBaseUri[opts.AzureEnvironment]
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
	cmd.Flags().StringVar(&opts.AzureEnvironment, "azure-environment", "", "Set only if you are using an isolated Azure Environment. Configure isolated Azure Environment. Valid option are AzureChinaCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernment")
	cmd.Flags().StringVar(&opts.ADEndpointBaseUrl, "ad-endpoint-base-uri", "", "Set this only if you need to override the default Active Directory Endpoint.")
	cmd.Flags().StringVar(&opts.RMBaseUri, "resource-management-base-uri", "", "Set this only if you need to override the default Resource Management Endpoint.")
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
	if err != nil {
		return err
	}
	servicePrincipalAccount.Description = opts.Description
	servicePrincipalAccount.AzureEnvironment = opts.AzureEnvironment
	servicePrincipalAccount.ResourceManagerEndpoint = opts.RMBaseUri
	servicePrincipalAccount.AuthenticationEndpoint = opts.ADEndpointBaseUrl

	createdAccount, err = opts.Octopus.Accounts.Add(servicePrincipalAccount)
	if err != nil {
		return err
	}

	opts.Spinner.Start()
	_, err = fmt.Fprintf(opts.Writer, "Successfully created Azure Account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
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
			Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
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

	if opts.AzureEnvironment == "" {
		var shouldConfigureAzureEnvironment bool
		if err := opts.Ask(&survey.Confirm{
			Message: "Configure isolated Azure Environment connection.",
			Default: false,
		}, &shouldConfigureAzureEnvironment); err != nil {
			return err
		}
		if shouldConfigureAzureEnvironment {
			envMapKeys := make([]string, 0, len(azureEnvMap))
			for keys := range azureEnvMap {
				envMapKeys = append(envMapKeys, keys)
			}
			if err := opts.Ask(&survey.Select{
				Message: "Azure Environment",
				Options: envMapKeys,
				Default: "Global Cloud (Default)",
			}, &opts.AzureEnvironment); err != nil {
				return err
			}
			opts.AzureEnvironment = azureEnvMap[opts.AzureEnvironment]
		}
	}

	if opts.AzureEnvironment != "" {
		if opts.ADEndpointBaseUrl == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Active Directory endpoint base URI",
				Default: azureADEndpointBaseUri[opts.AzureEnvironment],
				Help:    "Set this only if you need to override the default Active Directory Endpoint. In most cases you should leave the pre-populated value as is.",
			}, &opts.ADEndpointBaseUrl); err != nil {
				return err
			}
		}
		if opts.RMBaseUri == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Resource Management Base URI",
				Default: azureResourceManagementBaseUri[opts.AzureEnvironment],
				Help:    "Set this only if you need to override the default Resource Management Endpoint. In most cases you should leave the pre-populated value as is.",
			}, &opts.RMBaseUri); err != nil {
				return err
			}
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
