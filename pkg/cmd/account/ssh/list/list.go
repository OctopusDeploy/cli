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
		Short: "List ssh accounts",
		Long:  "List SSH Key Pair accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account ssh list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			return listSshAccounts(client, cmd)
		},
	}

	return cmd
}

func listSshAccounts(client *client.Client, cmd *cobra.Command) error {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeSSHKeyPair,
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
			acc := item.(*accounts.SSHKeyAccount)
			return &struct {
				Id       string
				Name     string
				Slug     string
				Username string
			}{
				Id:       acc.GetID(),
				Name:     acc.GetName(),
				Slug:     acc.GetSlug(),
				Username: acc.Username,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SLUG", "USERNAME"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.SSHKeyAccount)
				return []string{output.Bold(acc.GetName()), acc.GetSlug(), acc.Username}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
