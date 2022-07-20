package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cliOutput"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
			client, err := client.Get(true)
			if err != nil {
				return err
			}

			allEnvs, err := client.Environments.GetAll()
			if err != nil {
				return err
			}

			return cliOutput.PrintArray(allEnvs, cmd,
				func(e *environments.Environment) any {
					return cliOutput.IdAndName{Id: e.GetID(), Name: e.Name}
				}, func(e *environments.Environment) string {
					return fmt.Sprintf("%s\t%s\t%s", e.GetID(), e.Name, e.Description)
				})
		},
	}

	return cmd
}
