package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
	"io"
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

			return output.PrintArray(allEnvs, cmd, output.Mappers[*environments.Environment]{
				Json: func(item *environments.Environment) any {
					return output.IdAndName{Id: item.GetID(), Name: item.Name}
				},
				Table: output.TableDefinition[*environments.Environment]{
					Header: []string{"ID", "NAME", "DESCRIPTION"},
					Row: func(item *environments.Environment, io io.Writer) []string {
						return []string{item.GetID(), output.Bold(item.Name), item.Description}
					},
				},
				Basic: func(item *environments.Environment) string {
					return item.Name
				},
			})
		},
	}

	return cmd
}
