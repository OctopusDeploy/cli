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
			client, err := client.GetSpacedClient()
			if err != nil {
				return err
			}

			allAccounts, err := client.Accounts.GetAll()
			if err != nil {
				return err
			}

			type AccountJson struct {
				Id   string
				Name string
				Type string
				// TODO should we list type-specific fields here? (e.g. an Azure DevOps account may have different attributes than a Username)
			}

			return output.PrintArray(allAccounts, cmd, output.Mappers[accounts.IAccount]{
				Json: func(item accounts.IAccount) any {
					return AccountJson{Id: item.GetID(), Name: item.GetName(), Type: string(item.GetAccountType())}
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
