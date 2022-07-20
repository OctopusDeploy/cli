package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdList(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments in an instance of Octopus Deploy",
		Long:  "List environments in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.Get(false)
			if err != nil {
				return err
			}

			environments, err := client.Environments.GetAll()
			if err != nil {
				return err
			}

			for _, e := range environments {
				fmt.Printf("%s\t%s\t%s\n", e.GetID(), e.Name, e.Description)
			}

			return nil
		},
	}

	return cmd
}
