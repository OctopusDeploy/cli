package create

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"os"
	"strings"

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
	*cmd.Dependencies
	selectors.GetAllEnvironmentsCallback
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

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(dependencies.Client)
		},
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
	createFlags := NewCreateFlags()
	descriptionFilePath := ""

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an Azure subscription account",
		Long:    "Create an Azure subscription account in Octopus Deploy",
		Example: heredoc.Docf("$ %s account azure create", constants.ExecutableName),
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
	flags.StringVar(&createFlags.SubscriptionID.Value, createFlags.SubscriptionID.Name, "", "Your Azure subscription ID.")
	flags.StringVar(&createFlags.TenantID.Value, createFlags.TenantID.Name, "", "Your Azure Active Directory Tenant ID.")
	flags.StringVar(&createFlags.ApplicationID.Value, createFlags.ApplicationID.Name, "", "Your Azure Active Directory Application ID.")
	flags.StringVar(&createFlags.ApplicationPasswordKey.Value, createFlags.ApplicationPasswordKey.Name, "", "The password for the Azure Active Directory application.")
	flags.StringArrayVarP(&createFlags.Environments.Value, createFlags.Environments.Name, "e", nil, "The environments that are allowed to use this account")
	flags.StringVar(&createFlags.AzureEnvironment.Value, createFlags.AzureEnvironment.Name, "", "Set only if you are using an isolated Azure Environment. Configure isolated Azure Environment. Valid option are AzureChinaCloud, AzureChinaCloud, AzureGermanCloud or AzureUSGovernment")
	flags.StringVar(&createFlags.ADEndpointBaseUrl.Value, createFlags.ADEndpointBaseUrl.Name, "", "Set this only if you need to override the default Active Directory Endpoint.")
	flags.StringVar(&createFlags.RMBaseUri.Value, createFlags.RMBaseUri.Name, "", "Set this only if you need to override the default Resource Management Endpoint.")
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

	createdAccount, err = opts.Client.Accounts.Add(servicePrincipalAccount)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created Azure account %s %s.\n", createdAccount.GetName(), output.Dimf("(%s)", createdAccount.GetSlug()))
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
			opts.SubscriptionID,
			opts.TenantID,
			opts.ApplicationID,
			opts.ApplicationPasswordKey,
			opts.AzureEnvironment,
			opts.ADEndpointBaseUrl,
			opts.RMBaseUri,
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
