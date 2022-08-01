package list

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments in an instance of Octopus Deploy",
		Long:  "List environments in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			f.Spinner().Start()
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}

			envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
			if err != nil {
				f.Spinner().Stop()
				return err
			}
			allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
			if err != nil {
				f.Spinner().Stop()
				return err
			}
			f.Spinner().Stop()

			return output.PrintArray(allEnvs, cmd, output.Mappers[*environments.Environment]{
				Json: func(item *environments.Environment) any {
					return output.IdAndName{Id: item.GetID(), Name: item.Name}
				},
				Table: output.TableDefinition[*environments.Environment]{
					Header: []string{"NAME", "GUIDED FAILURE"},
					Row: func(item *environments.Environment) []string {

						return []string{output.Bold(item.Name), strconv.FormatBool(item.UseGuidedFailure)}
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
