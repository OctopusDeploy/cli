package shared

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/certificates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
)

type GetAccountsByTypeCallback func(accountType accounts.AccountType) ([]accounts.IAccount, error)
type GetAllWorkerPoolsCallback func() ([]*workerpools.WorkerPoolListResult, error)
type GetAllCertificatesCallback func() ([]*certificates.CertificateResource, error)

type VariableCallbacks struct {
	GetAccountsByType  GetAccountsByTypeCallback
	GetAllWorkerPools  GetAllWorkerPoolsCallback
	GetAllCertificates GetAllCertificatesCallback
}

func NewVariableCallbacks(dependencies *cmd.Dependencies) *VariableCallbacks {
	return &VariableCallbacks{
		GetAccountsByType: func(accountType accounts.AccountType) ([]accounts.IAccount, error) {
			return getAccountsByType(dependencies.Client, accountType)
		},
		GetAllWorkerPools: func() ([]*workerpools.WorkerPoolListResult, error) {
			return getAllWorkerPools(dependencies.Client)
		},
		GetAllCertificates: func() ([]*certificates.CertificateResource, error) {
			return getAllCertificates(dependencies.Client)
		},
	}
}

func PromptValue(ask question.Asker, variableType string, callbacks *VariableCallbacks) (string, error) {
	var value string
	switch variableType {
	case "String":
		if err := ask(&survey.Input{
			Message: "Value",
		}, &value); err != nil {
			return "", err
		}
		return value, nil
	case "Sensitive":
		if err := ask(&survey.Password{
			Message: "Value",
		}, &value); err != nil {
			return "", err
		}
		return value, nil
	case "AmazonWebServicesAccount", "AzureAccount", "GoogleCloudAccount":
		accountType, err := mapVariableTypeToAccountType(variableType)
		if err != nil {
			return "", err
		}
		accountsByType, err := callbacks.GetAccountsByType(accountType)
		if err != nil {
			return "", err
		}

		selectedValue, err := selectors.ByName(ask, accountsByType, "Value")
		if err != nil {
			return "", err
		}
		return selectedValue.GetName(), nil
	case "WorkerPool":
		workerPools, err := callbacks.GetAllWorkerPools()
		if err != nil {
			return "", err
		}
		selectedValue, err := selectors.Select(
			ask,
			"Value",
			func() ([]*workerpools.WorkerPoolListResult, error) { return workerPools, nil },
			func(item *workerpools.WorkerPoolListResult) string { return item.Name })
		if err != nil {
			return "", err
		}
		return selectedValue.Name, nil
	case "Certificate":
		allCerts, err := callbacks.GetAllCertificates()
		if err != nil {
			return "", err
		}
		selectedValue, err := selectors.Select(
			ask,
			"Value",
			func() ([]*certificates.CertificateResource, error) { return allCerts, nil },
			func(item *certificates.CertificateResource) string { return item.Name })
		if err != nil {
			return "", err
		}
		return selectedValue.Name, nil
	}

	return "", fmt.Errorf("error getting value")
}

func PromptScopes(asker question.Asker, projectVariables variables.VariableSet, flags *ScopeFlags, isPrompted bool) error {
	var err error
	if util.Empty(flags.EnvironmentsScopes.Value) {
		flags.EnvironmentsScopes.Value, err = PromptScope(asker, "Environment", projectVariables.ScopeValues.Environments)
		if err != nil {
			return err
		}
	}

	if util.Empty(flags.ProcessScopes.Value) {
		flags.ProcessScopes.Value, err = PromptScope(asker, "Process", ConvertProcessScopesToReference(projectVariables.ScopeValues.Processes))
		if err != nil {
			return err
		}
	}

	if !isPrompted {
		if util.Empty(flags.ChannelScopes.Value) {
			flags.ChannelScopes.Value, err = PromptScope(asker, "Channel", projectVariables.ScopeValues.Channels)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.TargetScopes.Value) {
			flags.TargetScopes.Value, err = PromptScope(asker, "Target", projectVariables.ScopeValues.Machines)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.RoleScopes.Value) {
			flags.RoleScopes.Value, err = PromptScope(asker, "Role", projectVariables.ScopeValues.Roles)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.TagScopes.Value) {
			flags.TagScopes.Value, err = PromptScope(asker, "Tag", projectVariables.ScopeValues.TenantTags)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.StepScopes.Value) {
			flags.StepScopes.Value, err = PromptScope(asker, "Step", projectVariables.ScopeValues.Actions)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func PromptScope(ask question.Asker, scopeDescription string, items []*resources.ReferenceDataItem) ([]string, error) {
	if util.Empty(items) {
		return nil, nil
	}
	var selectedItems []string
	err := ask(&survey.MultiSelect{
		Message: fmt.Sprintf("%s scope", scopeDescription),
		Options: util.SliceTransform(items, func(i *resources.ReferenceDataItem) string { return i.Name }),
	}, &selectedItems)

	if err != nil {
		return nil, err
	}

	return selectedItems, nil
}

func mapVariableTypeToAccountType(variableType string) (accounts.AccountType, error) {
	switch variableType {
	case "AmazonWebServicesAccount":
		return accounts.AccountTypeAmazonWebServicesAccount, nil
	case "AzureAccount":
		return accounts.AccountTypeAzureServicePrincipal, nil
	case "GoogleCloudAccount":
		return accounts.AccountTypeGoogleCloudPlatformAccount, nil
	default:
		return accounts.AccountTypeNone, fmt.Errorf("variable type '%s' is not a valid account variable type", variableType)

	}
}

func getAccountsByType(client *client.Client, accountType accounts.AccountType) ([]accounts.IAccount, error) {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accountType,
	})
	if err != nil {
		return nil, err
	}
	items, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		return nil, err
	}
	return items, nil
}

func getAllCertificates(client *client.Client) ([]*certificates.CertificateResource, error) {
	certs, err := client.Certificates.Get(certificates.CertificatesQuery{})
	if err != nil {
		return nil, err
	}
	return certs.GetAllPages(client.Sling())
}

func getAllWorkerPools(client *client.Client) ([]*workerpools.WorkerPoolListResult, error) {
	res, err := client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
