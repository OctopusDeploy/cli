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
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type CreateFlags struct {
	Name                   *flag.Flag[string]
	Description            *flag.Flag[string]
	Environments           *flag.Flag[[]string]
	SubscriptionID         *flag.Flag[string]
	TenantID               *flag.Flag[string]
	ApplicationID          *flag.Flag[string]
	ApplicationPasswordKey *flag.Flag[string]
	AzureEnvironment       *flag.Flag[string]
	ADEndpointBaseUrl      *flag.Flag[string]
	RMBaseUri              *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	Writer   io.Writer
	Octopus  *client.Client
	Ask      question.Asker
	Spinner  factory.Spinner
	Space    string
	NoPrompt bool
	Host     string
	CmdPath  string
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                   flag.New[string]("name", false),
		Description:            flag.New[string]("description", false),
		Environments:           flag.New[[]string]("environment", false),
		SubscriptionID:         flag.New[string]("subscription-id", false),
		TenantID:               flag.New[string]("tenant-id", false),
		ApplicationID:          flag.New[string]("application-id", false),
		ApplicationPasswordKey: flag.New[string]("application-key", true),
		AzureEnvironment:       flag.New[string]("azure-environment", false),
		ADEndpointBaseUrl:      flag.New[string]("ad-endpoint-base-uri", false),
		RMBaseUri:              flag.New[string]("resource-management-base-uri", false),
	}
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
		Ask:         f.Ask,
		Spinner:     f.Spinner(),
		CreateFlags: NewCreateFlags(),
	}
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an Azure account",
		Long:  "Creates an Azure account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account azure create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.CmdPath = cmd.CommandPath()
			opts.Host = f.GetCurrentHost()
			opts.Space = f.GetCurrentSpace().GetID()
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
				opts.Description.Value = string(data)
			}
			opts.NoPrompt = !f.IsPromptEnabled()

			if opts.SubscriptionID.Value != "" {
				if err := validation.IsUuid(opts.SubscriptionID); err != nil {
					return err
				}
			}
			if opts.TenantID.Value != "" {
				if err := validation.IsUuid(opts.TenantID); err != nil {
					return err
				}
			}
			if opts.ApplicationID.Value != "" {
				if err := validation.IsUuid(opts.ApplicationID); err != nil {
					return err
				}
			}
			if opts.AzureEnvironment.Value != "" {
				isAzureEnvCorrect := false
				for _, value := range azureEnvMap {
					if strings.EqualFold(value, opts.AzureEnvironment.Value) {
						opts.AzureEnvironment.Value = value
						isAzureEnvCorrect = true
						break
					}
				}
				if !isAzureEnvCorrect {
					return fmt.Errorf("the Azure environment %s is not correct, please use AzureChinaCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernment", opts.AzureEnvironment.Value)
				}
				if opts.RMBaseUri.Value == "" && opts.NoPrompt {
					opts.RMBaseUri.Value = azureResourceManagementBaseUri[opts.AzureEnvironment.Value]
				}
				if opts.ADEndpointBaseUrl.Value == "" && opts.NoPrompt {
					opts.ADEndpointBaseUrl.Value = azureADEndpointBaseUri[opts.AzureEnvironment.Value]
				}
			}
			if opts.Environments.Value != nil {
				opts.Environments.Value, err = helper.ResolveEnvironmentNames(opts.Environments.Value, opts.Octopus, opts.Spinner)
				if err != nil {
					return err
				}
			}
			return CreateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name.Value, opts.Name.Name, "n", "", "A short, memorable, unique name for this account.")
	cmd.Flags().StringVarP(&opts.Description.Value, opts.Description.Value, "d", "", "A summary explaining the use of the account to other users.")
	cmd.Flags().StringVar(&opts.SubscriptionID.Value, opts.SubscriptionID.Name, "", "Your Azure subscription ID.")
	cmd.Flags().StringVar(&opts.TenantID.Value, opts.TenantID.Name, "", "Your Azure Active Directory Tenant ID.")
	cmd.Flags().StringVar(&opts.ApplicationID.Value, opts.ApplicationID.Name, "", "Your Azure Active Directory Application ID.")
	cmd.Flags().StringVar(&opts.ApplicationPasswordKey.Value, opts.ApplicationPasswordKey.Name, "", "The password for the Azure Active Directory application.")
	cmd.Flags().StringArrayVarP(&opts.Environments.Value, opts.Environments.Name, "e", nil, "The environments that are allowed to use this account")
	cmd.Flags().StringVar(&opts.AzureEnvironment.Value, opts.AzureEnvironment.Name, "", "Set only if you are using an isolated Azure Environment. Configure isolated Azure Environment. Valid option are AzureChinaCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernment")
	cmd.Flags().StringVar(&opts.ADEndpointBaseUrl.Value, opts.ADEndpointBaseUrl.Name, "", "Set this only if you need to override the default Active Directory Endpoint.")
	cmd.Flags().StringVar(&opts.RMBaseUri.Value, opts.RMBaseUri.Name, "", "Set this only if you need to override the default Resource Management Endpoint.")
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
	subId, err := uuid.Parse(opts.SubscriptionID.Value)
	if err != nil {
		return err
	}
	tenantID, err := uuid.Parse(opts.TenantID.Value)
	if err != nil {
		return err
	}
	appID, err := uuid.Parse(opts.ApplicationID.Value)
	if err != nil {
		return err
	}
	servicePrincipalAccount, err := accounts.NewAzureServicePrincipalAccount(
		opts.Name.Value,
		subId,
		tenantID,
		appID,
		core.NewSensitiveValue(opts.ApplicationPasswordKey.Value),
	)
	if err != nil {
		return err
	}
	servicePrincipalAccount.Description = opts.Description.Value
	servicePrincipalAccount.AzureEnvironment = opts.AzureEnvironment.Value
	servicePrincipalAccount.ResourceManagerEndpoint = opts.RMBaseUri.Value
	servicePrincipalAccount.AuthenticationEndpoint = opts.ADEndpointBaseUrl.Value

	opts.Spinner.Start()
	createdAccount, err = opts.Octopus.Accounts.Add(servicePrincipalAccount)
	opts.Spinner.Stop()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Writer, "Successfully created Azure account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetID()))
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/infrastructure/accounts/%s", opts.Host, opts.Space, createdAccount.GetID())
	fmt.Fprintf(opts.Writer, "\nView this account on Octopus Deploy: %s\n", link)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath,
			opts.Name,
			opts.Description,
			opts.Environments,
			opts.SubscriptionID,
			opts.TenantID,
			opts.ApplicationID,
			opts.ApplicationPasswordKey,
			opts.AzureEnvironment,
			opts.ADEndpointBaseUrl,
			opts.RMBaseUri,
		)
		fmt.Fprintf(opts.Writer, "\nAutomation Command: %s\n", autoCmd)
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

	if opts.SubscriptionID.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Subscription ID",
			Help:    "Your Azure subscription ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.SubscriptionID.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.TenantID.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Tenant ID",
			Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.TenantID.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.ApplicationID.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Application ID",
			Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
		}, &opts.ApplicationID.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			validation.IsUuid,
		))); err != nil {
			return err
		}
	}

	if opts.ApplicationPasswordKey.Value == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Application Password / Key",
			Help:    "The password for the Azure Active Directory application. This value is known as Key in the Azure Portal, and Password in the API.",
		}, &opts.ApplicationPasswordKey.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.AzureEnvironment.Value == "" {
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
			}, &opts.AzureEnvironment.Value); err != nil {
				return err
			}
			opts.AzureEnvironment.Value = azureEnvMap[opts.AzureEnvironment.Value]
		}
	}

	if opts.AzureEnvironment.Value != "" {
		if opts.ADEndpointBaseUrl.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Active Directory endpoint base URI",
				Default: azureADEndpointBaseUri[opts.AzureEnvironment.Value],
				Help:    "Set this only if you need to override the default Active Directory Endpoint. In most cases you should leave the pre-populated value as is.",
			}, &opts.ADEndpointBaseUrl.Value); err != nil {
				return err
			}
		}
		if opts.RMBaseUri.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Resource Management Base URI",
				Default: azureResourceManagementBaseUri[opts.AzureEnvironment.Value],
				Help:    "Set this only if you need to override the default Resource Management Endpoint. In most cases you should leave the pre-populated value as is.",
			}, &opts.RMBaseUri.Value); err != nil {
				return err
			}
		}
	}

	if opts.Environments.Value == nil {
		environmentIDs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.Octopus, opts.Spinner,
			"Choose the environments that are allowed to use this account.\n"+
				output.Dim("If nothing is selected, the account can be used for deployments to any environment."))
		if err != nil {
			return err
		}
		opts.Environments.Value = environmentIDs
	}
	return nil
}
