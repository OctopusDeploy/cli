package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/spf13/cobra"
)

func NewCmdList(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts in an instance of Octopus Deploy",
		Long:  "List accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.Get(true)
			if err != nil {
				return err
			}

			allAccounts, err := client.Accounts.GetAll()
			if err != nil {
				return err
			}

			return output.PrintArray(allAccounts, cmd, output.Mappers[accounts.IAccount]{
				Json: func(item accounts.IAccount) any {
					return output.IdAndName{Id: item.GetID(), Name: item.GetName()}
				},
				Table: output.TableDefinition[accounts.IAccount]{
					Header: []string{"NAME", "TYPE"},
					Row: func(item accounts.IAccount) []string {
						return []string{output.Bold(item.GetName()), string(item.GetAccountType())}
					}},
				Basic: func(item accounts.IAccount) string {
					return item.GetName()
				},
			})
		},
	}

	return cmd
}
