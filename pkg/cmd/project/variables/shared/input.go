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
type GetProjectVariablesCallback func(projectId string) (*variables.VariableSet, error)
type GetVariableByIdCallback func(ownerId, variableId string) (*variables.Variable, error)
type GetAllLibraryVariableSetsCallback func() ([]*variables.LibraryVariableSet, error)

type VariableCallbacks struct {
	GetAccountsByType   GetAccountsByTypeCallback
	GetAllWorkerPools   GetAllWorkerPoolsCallback
	GetAllCertificates  GetAllCertificatesCallback
	GetProjectVariables GetProjectVariablesCallback
	GetVariableById     GetVariableByIdCallback
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
		GetProjectVariables: func(projectId string) (*variables.VariableSet, error) {
			return getProjectVariables(dependencies.Client, projectId)
		},
		GetVariableById: func(ownerId, variableId string) (*variables.Variable, error) {
			return getVariableById(dependencies.Client, ownerId, variableId)
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

func PromptScopes(asker question.Asker, projectVariables *variables.VariableSet, flags *ScopeFlags, isPrompted bool) error {
	var err error
	if util.Empty(flags.EnvironmentsScopes.Value) {
		flags.EnvironmentsScopes.Value, err = PromptScope(asker, "Environment", projectVariables.ScopeValues.Environments, nil)
		if err != nil {
			return err
		}
	}

	flags.ProcessScopes.Value, err = PromptScope(asker, "Process", ConvertProcessScopesToReference(projectVariables.ScopeValues.Processes), nil)
	if err != nil {
		return err
	}

	if !isPrompted {
		flags.ChannelScopes.Value, err = PromptScope(asker, "Channel", projectVariables.ScopeValues.Channels, nil)
		if err != nil {
			return err
		}

		flags.TargetScopes.Value, err = PromptScope(asker, "Target", projectVariables.ScopeValues.Machines, nil)
		if err != nil {
			return err
		}

		flags.RoleScopes.Value, err = PromptScope(asker, "Role", projectVariables.ScopeValues.Roles, nil)
		if err != nil {
			return err
		}

		flags.TagScopes.Value, err = PromptScope(asker, "Tag", projectVariables.ScopeValues.TenantTags, func(i *resources.ReferenceDataItem) string { return i.ID })
		if err != nil {
			return err
		}

		flags.StepScopes.Value, err = PromptScope(asker, "Step", projectVariables.ScopeValues.Actions, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func PromptScope(ask question.Asker, scopeDescription string, items []*resources.ReferenceDataItem, displaySelector func(i *resources.ReferenceDataItem) string) ([]string, error) {
	if displaySelector == nil {
		displaySelector = func(i *resources.ReferenceDataItem) string { return i.Name }
	}
	if util.Empty(items) {
		return nil, nil
	}
	var selectedItems []string
	err := ask(&survey.MultiSelect{
		Message: fmt.Sprintf("%s scope", scopeDescription),
		Options: util.SliceTransform(items, displaySelector),
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

func getProjectVariables(client *client.Client, id string) (*variables.VariableSet, error) {
	variableSet, err := client.Variables.GetAll(id)
	return &variableSet, err
}

func getVariableById(client *client.Client, ownerId string, variableId string) (*variables.Variable, error) {
	return client.Variables.GetByID(ownerId, variableId)
}

func GetAllLibraryVariableSets(client *client.Client) ([]*variables.LibraryVariableSet, error) {
	res, err := client.LibraryVariableSets.GetAll()
	if err != nil {
		return nil, err
	}

	return util.SliceFilter(res, func(item *variables.LibraryVariableSet) bool { return item.ContentType == "Variables" }), nil
}
