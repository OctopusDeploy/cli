package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
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
			client, err := client.Get(false)
			if err != nil {
				return err
			}

			accounts, err := client.Accounts.GetAll()
			if err != nil {
				return err
			}

			for _, account := range accounts {
				fmt.Printf("%s\t%s\t%s\n", account.GetID(), account.GetName(), account.GetDescription())
			}

			return nil
		},
	}

	return cmd
}
