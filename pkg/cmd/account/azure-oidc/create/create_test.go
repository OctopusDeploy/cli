package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/account/azure-oidc/create"
	"github.com/OctopusDeploy/cli/test/testutil"
)

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "The Final Frontier"
	flags.Description.Value = "Where no person has gone before"
	flags.SubscriptionID.Value = "fec7f106-0e8d-4f27-ac37-530add5e4557"
	flags.TenantID.Value = "fec7f106-0e8d-4f27-ac37-530add5e4557"
	flags.ApplicationID.Value = "fec7f106-0e8d-4f27-ac37-530add5e4557"
	flags.AzureEnvironment.Value = "GlobalCloud"
	flags.ADEndpointBaseUrl.Value = "https://something.windows.net"
	flags.RMBaseUri.Value = "https://rm.microsoft.net"
	flags.ExecutionSubjectKeys.Value = []string{"space"}
	flags.HealthSubjectKeys.Value = []string{"space"}
	flags.AccountTestSubjectKeys.Value = []string{"space"}
	flags.Environments.Value = []string{"dev"}

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
	}
	create.PromptMissing(opts)
	checkRemainingPrompts()
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this account.", "oidc account"),
		testutil.NewInputPrompt("Subscription ID", "Your Azure Subscription ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.", "9843a42e-4fe9-4902-8b38-94257ee2b8d7"),
		testutil.NewInputPrompt("Tenant ID", "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.", "c0441a23-3450-41f0-acf2-75c7209b57cc"),
		testutil.NewInputPrompt("Application ID", "Your Azure Active Directory Application ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.", "ffe0aca0-91a4-4b2e-a754-26d766b24bec"),
		testutil.NewConfirmPromptWithDefault("Configure isolated Azure Environment connection.", "", true, false),
		testutil.NewSelectPromptWithDefault("Azure Environment", "", []string{"Global Cloud (Default)", "China Cloud", "German Cloud", "US Government"}, "Global Cloud (Default)", "Global Cloud (Default)"),
		testutil.NewInputPromptWithDefault("Active Directory endpoint base URI", "Set this only if you need to override the default Active Directory Endpoint. In most cases you should leave the pre-populated value as is.", "https://login.microsoftonline.com/", ""),
		testutil.NewInputPromptWithDefault("Resource Management Base URI", "Set this only if you need to override the default Resource Management Endpoint. In most cases you should leave the pre-populated value as is.", "https://management.azure.com/", ""),
		testutil.NewMultiSelectPrompt("Deployment and Runbook subject keys", "", []string{"space", "environment", "project", "tenant", "runbook", "account", "type"}, []string{"space", "type"}),
		testutil.NewMultiSelectPrompt("Health check subject keys", "", []string{"space", "target", "account", "type"}, []string{"space", "target"}),
		testutil.NewMultiSelectPrompt("Account test subject keys", "", []string{"space", "account", "type"}, []string{"space", "account"}),
		testutil.NewMultiSelectPrompt("Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.", "", []string{"testenv"}, []string{"testenv"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Description.Value = "the description" // this is due the input mocking not support OctoEditor

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{fixtures.NewEnvironment("Spaces-1", "Environments-1", "testenv")}, nil
		},
	}
	_ = create.PromptMissing(opts)

	assert.Equal(t, "oidc account", flags.Name.Value)
	assert.Equal(t, "9843a42e-4fe9-4902-8b38-94257ee2b8d7", flags.SubscriptionID.Value)
	assert.Equal(t, "c0441a23-3450-41f0-acf2-75c7209b57cc", flags.TenantID.Value)
	assert.Equal(t, "ffe0aca0-91a4-4b2e-a754-26d766b24bec", flags.ApplicationID.Value)
	assert.Equal(t, "AzureCloud", flags.AzureEnvironment.Value)
	assert.Equal(t, "", flags.ADEndpointBaseUrl.Value)
	assert.Equal(t, "", flags.RMBaseUri.Value)
	assert.Equal(t, []string{"space", "type"}, flags.ExecutionSubjectKeys.Value)
	assert.Equal(t, []string{"space", "target"}, flags.HealthSubjectKeys.Value)
	assert.Equal(t, []string{"space", "account"}, flags.AccountTestSubjectKeys.Value)
	assert.Equal(t, []string{"Environments-1"}, flags.Environments.Value)
	checkRemainingPrompts()
}
