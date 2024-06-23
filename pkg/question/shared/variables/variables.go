package variables

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/certificates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"strings"
)

type VariableType string

const (
	VariableTypeString             = VariableType("String")
	VariableTypeSensitive          = VariableType("Sensitive")
	VariableTypeAwsAccount         = VariableType("AmazonWebServicesAccount")
	VariableTypeAzureAccount       = VariableType("AzureAccount")
	VariableTypeGoogleCloudAccount = VariableType("GoogleCloudAccount")
	VariableTypeWorkerPool         = VariableType("WorkerPool")
	VariableTypeCertificate        = VariableType("Certificate")
	VariableTypeBoolean            = VariableType("Boolean")
	VariableTypeSelect             = VariableType("Select")
)

type GetAccountsByTypeCallback func(accountType accounts.AccountType) ([]accounts.IAccount, error)
type GetAllWorkerPoolsCallback func() ([]*workerpools.WorkerPoolListResult, error)
type GetAllCertificatesCallback func() ([]*certificates.CertificateResource, error)
type GetProjectVariablesCallback func(projectId string) (*variables.VariableSet, error)
type GetProjectVariablesByGitRefCallback func(spaceId string, projectId string, gitRef string) (*variables.VariableSet, error)
type GetTenantVariablesCallback func(tenant *tenants.Tenant) (*variables.TenantVariables, error)
type GetVariableByIdCallback func(ownerId, variableId string) (*variables.Variable, error)
type GetAllLibraryVariableSetsCallback func() ([]*variables.LibraryVariableSet, error)

type VariableCallbacks struct {
	GetAccountsByType           GetAccountsByTypeCallback
	GetAllWorkerPools           GetAllWorkerPoolsCallback
	GetAllCertificates          GetAllCertificatesCallback
	GetProjectVariables         GetProjectVariablesCallback
	GetProjectVariablesByGitRef GetProjectVariablesByGitRefCallback
	GetVariableById             GetVariableByIdCallback
	GetTenantVariables          GetTenantVariablesCallback
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
		GetProjectVariablesByGitRef: func(spaceId string, projectId string, gitRef string) (*variables.VariableSet, error) {
			return getProjectVariablesByGitRef(dependencies.Client, spaceId, projectId, gitRef)
		},
		GetTenantVariables: func(tenant *tenants.Tenant) (*variables.TenantVariables, error) {
			return getTenantVariables(dependencies.Client, tenant)
		},
		GetVariableById: func(ownerId, variableId string) (*variables.Variable, error) {
			return getVariableById(dependencies.Client, ownerId, variableId)
		},
	}
}

func PromptValue(ask question.Asker, variableType VariableType, callbacks *VariableCallbacks, template *actiontemplates.ActionTemplateParameter) (string, error) {
	var value string
	switch variableType {
	case VariableTypeString:
		if err := ask(&survey.Input{
			Message: "Value",
		}, &value); err != nil {
			return "", err
		}
		return value, nil
	case VariableTypeSensitive:
		if err := ask(&survey.Password{
			Message: "Value",
		}, &value); err != nil {
			return "", err
		}
		return value, nil
	case VariableTypeAwsAccount, VariableTypeAzureAccount, VariableTypeGoogleCloudAccount:
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
	case VariableTypeWorkerPool:
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
	case VariableTypeCertificate:
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
	case VariableTypeBoolean:
		var response string
		err := ask(&survey.Select{
			Message: "Select value",
			Options: []string{"True", "False"}, // Yes/No would read more nicely, but doesn't fit well with cmdline which expects True/False
		}, &response)
		return response, err
	case VariableTypeSelect:
		response, err := selectors.SelectOptions(ask, "Selection option", func() []*selectors.SelectOption[string] { return GetSelectOptions(template) })
		if err != nil {
			return "", err
		}
		return response.Value, nil
	}

	return "", fmt.Errorf("error getting value")
}

func mapVariableTypeToAccountType(variableType VariableType) (accounts.AccountType, error) {
	switch variableType {
	case VariableTypeAwsAccount:
		return accounts.AccountTypeAmazonWebServicesAccount, nil
	case VariableTypeAzureAccount:
		return accounts.AccountTypeAzureServicePrincipal, nil
	case VariableTypeGoogleCloudAccount:
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

func getProjectVariablesByGitRef(client *client.Client, spaceId string, projectId string, gitRef string) (*variables.VariableSet, error) {
	variableSet, err := client.ProjectVariables.GetAllByGitRef(spaceId, projectId, gitRef)
	return variableSet, err
}

func getTenantVariables(client *client.Client, tenant *tenants.Tenant) (*variables.TenantVariables, error) {
	tenantVariables, err := client.Tenants.GetVariables(tenant)
	return tenantVariables, err
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

func GetSelectOptions(t *actiontemplates.ActionTemplateParameter) []*selectors.SelectOption[string] {
	var selectionOptions []*selectors.SelectOption[string]
	options := t.DisplaySettings["Octopus.SelectOptions"]
	for _, l := range strings.Split(options, "\n") {
		o := strings.Split(l, "|")
		selectionOptions = append(selectionOptions, selectors.NewSelectOption[string](o[0], o[1]))
	}

	return selectionOptions
}
