package list

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
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
		Short: "List environments",
		Long:  "List environments in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s environment list
			$ %[1]s environment ls"
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
			if err != nil {
				return err
			}
			allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
			if err != nil {
				return err
			}

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
