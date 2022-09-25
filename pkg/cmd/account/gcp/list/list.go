package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List gcp accounts",
		Long:  "List Google Cloud accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account gcp list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			return listGcpAccounts(client, cmd)
		},
	}

	return cmd
}

func listGcpAccounts(client *client.Client, cmd *cobra.Command) error {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeGoogleCloudPlatformAccount,
	})
	if err != nil {
		return err
	}
	items, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		return err
	}

	output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
		Json: func(item accounts.IAccount) any {
			acc := item.(*accounts.GoogleCloudPlatformAccount)
			return &struct {
				Id   string
				Name string
				Slug string
			}{
				Id:   acc.GetID(),
				Name: acc.GetName(),
				Slug: acc.GetSlug(),
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SLUG"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.GoogleCloudPlatformAccount)
				return []string{output.Bold(acc.GetName()), acc.GetSlug()}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
