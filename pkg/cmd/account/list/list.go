package list

import (
	"fmt"

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

			return output.PrintArray(allAccounts, cmd,
				func(item accounts.IAccount) any {
					return output.IdAndName{Id: item.GetID(), Name: item.GetName()}
				}, func(e accounts.IAccount) string {
					return fmt.Sprintf("%s\t%s\t%s", e.GetID(), e.GetName(), e.GetDescription())
				})
		},
	}

	return cmd
}
