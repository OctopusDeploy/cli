package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Azure subscription accounts",
		Long:    "List Azure subscription accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account azure list", constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}
			return listAzureAccounts(client, cmd)
		},
	}

	return cmd
}

func listAzureAccounts(client *client.Client, cmd *cobra.Command) error {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeAzureServicePrincipal,
	})
	if err != nil {
		return err
	}
	items, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		return err
	}

	azureEnvMap := map[string]string{
		"":                  "Global Cloud",
		"AzureCloud":        "Global Cloud",
		"AzureChinaCloud":   "China Cloud",
		"AzureGermanCloud":  "German Cloud",
		"AzureUSGovernment": "US Government",
	}

	output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
		Json: func(item accounts.IAccount) any {
			acc := item.(*accounts.AzureServicePrincipalAccount)
			return &struct {
				Id                 string
				Name               string
				Slug               string
				SubscriptionNumber string
				AccountType        string
				AzureEnvironment   string
			}{
				Id:                 acc.GetID(),
				Name:               acc.GetName(),
				Slug:               acc.GetSlug(),
				SubscriptionNumber: acc.SubscriptionID.String(),
				AccountType:        string(acc.AccountType),
				AzureEnvironment:   acc.AzureEnvironment,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SLUG", "SUBSCRIPTION ID", "AZURE ENVIRONMENT"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.AzureServicePrincipalAccount)
				return []string{
					output.Bold(acc.GetName()),
					acc.GetSlug(),
					acc.SubscriptionID.String(),
					azureEnvMap[acc.AzureEnvironment]}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
