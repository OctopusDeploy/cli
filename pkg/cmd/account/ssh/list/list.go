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
		Short: "List ssh accounts",
		Long:  "List SSH Key Pair accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account ssh list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			return listSshAccounts(client, cmd, f.Spinner())
		},
	}

	return cmd
}

func listSshAccounts(client *client.Client, cmd *cobra.Command, s *spinner.Spinner) error {
	s.Start()
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

	s.Stop()
	output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
		Json: func(item accounts.IAccount) any {
			acc := item.(*accounts.SSHKeyAccount)
			return &struct {
				Id       string
				Name     string
				Username string
			}{
				Id:       acc.GetID(),
				Name:     acc.GetName(),
				Username: acc.Username,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "USERNAME"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.SSHKeyAccount)
				return []string{output.Bold(acc.GetName()), acc.Username}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
