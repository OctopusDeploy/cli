package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts/azure"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForWebApp_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.WebApp.Value = "website"
	flags.ResourceGroup.Value = "rg1"
	flags.Slot.Value = "production"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureWebApps = func(a accounts.IAccount) ([]*azure.AzureWebApp, error) {
		return []*azure.AzureWebApp{
			{
				Name:          "website",
				Region:        "West US",
				ResourceGroup: "rg1",
			},
		}, nil
	}

	account, _ := accounts.NewAzureServicePrincipalAccount("azure", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))

	err := create.PromptForWebApp(opts, account)

	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForWebApp_NoFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the Azure Web App\n", "", []string{"website", "website 2"}, "website"),
		testutil.NewSelectPrompt("Select the Azure Web App slot\n", "", []string{"production", "test"}, "test"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	webapp := &azure.AzureWebApp{
		Name:          "website",
		Region:        "West US",
		ResourceGroup: "rg1",
	}

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureWebApps = func(a accounts.IAccount) ([]*azure.AzureWebApp, error) {
		return []*azure.AzureWebApp{
			webapp,
			{
				Name:          "website 2",
				Region:        "West US",
				ResourceGroup: "rg2",
			},
		}, nil
	}
	opts.GetAllAzureWebAppSlots = func(acc accounts.IAccount, wa *azure.AzureWebApp) ([]*azure.AzureWebAppSlot, error) {
		return []*azure.AzureWebAppSlot{
			{Name: "production"},
			{Name: "test"},
		}, nil
	}

	account, err := accounts.NewAzureServicePrincipalAccount("azure", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))

	err = create.PromptForWebApp(opts, account)

	checkRemainingPrompts()
	assert.NoError(t, err)

	assert.Equal(t, "website", flags.WebApp.Value)
	assert.Equal(t, "rg1", flags.ResourceGroup.Value)
	assert.Equal(t, "test", flags.Slot.Value)
}

func TestPromptForWebApp_NoSlotsAvailable(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.WebApp.Value = "website"
	flags.ResourceGroup.Value = "rg1"
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureWebApps = func(a accounts.IAccount) ([]*azure.AzureWebApp, error) {
		return []*azure.AzureWebApp{
			{
				Name:          "website",
				Region:        "West US",
				ResourceGroup: "rg1",
			},
		}, nil
	}

	opts.GetAllAzureWebAppSlots = func(acc accounts.IAccount, wa *azure.AzureWebApp) ([]*azure.AzureWebAppSlot, error) {
		return []*azure.AzureWebAppSlot{}, nil
	}

	account, _ := accounts.NewAzureServicePrincipalAccount("azure", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))

	err := create.PromptForWebApp(opts, account)

	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForAccount_FlagSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Account.Value = "Azure Account"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureAccounts = func() ([]accounts.IAccount, error) {
		a, _ := accounts.NewAzureServicePrincipalAccount("Azure account", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))
		return []accounts.IAccount{a}, nil
	}

	a, err := create.PromptForAccount(opts)

	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "Azure account", a.GetName())
	assert.Equal(t, "Azure account", opts.Account.Value)
}

func TestPromptForAccount_NoFlagSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the Azure Account to use\n", "", []string{"Azure account 1 (Azure Service Principal)", "Azure account the second (Azure Service Principal)"}, "Azure account the second (Azure Service Principal)"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureAccounts = func() ([]accounts.IAccount, error) {
		a1, _ := accounts.NewAzureServicePrincipalAccount("Azure account 1", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))
		a2, _ := accounts.NewAzureServicePrincipalAccount("Azure account the second", uuid.New(), uuid.New(), uuid.New(), core.NewSensitiveValue("password"))
		return []accounts.IAccount{a1, a2}, nil
	}

	a, err := create.PromptForAccount(opts)

	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "Azure account the second", a.GetName())
	assert.Equal(t, "Azure account the second", opts.Account.Value)
}

func TestPromptForAccount_OIDCFlagSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Account.Value = "Azure OIDC Account"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureAccounts = func() ([]accounts.IAccount, error) {
		oidcAccount, _ := accounts.NewAzureOIDCAccount("Azure OIDC Account", uuid.New(), uuid.New(), uuid.New())
		return []accounts.IAccount{oidcAccount}, nil
	}

	a, err := create.PromptForAccount(opts)

	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "Azure OIDC Account", a.GetName())
	assert.Equal(t, "Azure OIDC Account", opts.Account.Value)
}

func TestPromptForWebApp_OIDCAccount(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.WebApp.Value = "website"
	flags.ResourceGroup.Value = "rg1"
	flags.Slot.Value = "production"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAzureWebApps = func(a accounts.IAccount) ([]*azure.AzureWebApp, error) {
		return []*azure.AzureWebApp{
			{
				Name:          "website",
				Region:        "West US",
				ResourceGroup: "rg1",
			},
		}, nil
	}

	oidcAccount, _ := accounts.NewAzureOIDCAccount("Azure OIDC Account", uuid.New(), uuid.New(), uuid.New())

	err := create.PromptForWebApp(opts, oidcAccount)

	checkRemainingPrompts()
	assert.NoError(t, err)
}
