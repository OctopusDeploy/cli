package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List azure accounts",
		Long:  "List Azure service accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account azure list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			return listAzureAccounts(client, cmd, f.Spinner())
		},
	}

	return cmd
}

func listAzureAccounts(client *client.Client, cmd *cobra.Command, s *spinner.Spinner) error {
	s.Start()
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeAzureServicePrincipal,
	})
	if err != nil {
		s.Stop()
		return err
	}
	items, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		s.Stop()
		return err
	}
	accountResources, err = client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeAzureSubscription,
	})
	extraItems, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		s.Stop()
		return err
	}
	items = append(items, extraItems...)

	azureEnvMap := map[string]string{
		"":                  "Global Cloud",
		"AzureCloud":        "Global Cloud",
		"AzureChinaCloud":   "China Cloud",
		"AzureGermanCloud":  "German Cloud",
		"AzureUSGovernment": "US Government",
	}
	azureAccountTypeMap := map[string]string{
		"AzureSubscription":     "Subscription",
		"AzureServicePrincipal": "Service Principal",
	}
	s.Stop()

	output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
		Json: func(item accounts.IAccount) any {
			if acc, ok := item.(*accounts.AzureServicePrincipalAccount); ok {
				return &struct {
					Id                 string
					Name               string
					SubscriptionNumber string
					AccountType        string
					AzureEnvironment   string
				}{
					Id:                 acc.GetID(),
					Name:               acc.GetName(),
					SubscriptionNumber: acc.SubscriptionID.String(),
					AccountType:        string(acc.AccountType),
					AzureEnvironment:   acc.AzureEnvironment,
				}
			}
			acc := item.(*accounts.AzureSubscriptionAccount)
			return &struct {
				Id                 string
				Name               string
				SubscriptionNumber string
				AccountType        string
				AzureEnvironment   string
			}{
				Id:                 acc.GetID(),
				Name:               acc.GetName(),
				SubscriptionNumber: acc.SubscriptionID.String(),
				AccountType:        string(acc.AccountType),
				AzureEnvironment:   acc.AzureEnvironment,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SUBSCRIPTION ID", "AUTHENTICATION METHOD", "AZURE ENV"},
			Row: func(item accounts.IAccount) []string {
				if acc, ok := item.(*accounts.AzureServicePrincipalAccount); ok {
					return []string{
						output.Bold(acc.GetName()),
						acc.SubscriptionID.String(),
						azureAccountTypeMap[string(acc.AccountType)],
						azureEnvMap[acc.AzureEnvironment]}
				}
				acc := item.(*accounts.AzureSubscriptionAccount)
				return []string{
					output.Bold(acc.GetName()),
					acc.SubscriptionID.String(),
					azureAccountTypeMap[string(acc.AccountType)],
					azureEnvMap[acc.AzureEnvironment]}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
