package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Listening Tentacle workers",
		Long:    "List Listening Tentacle workers in Octopus Deploy",
		Aliases: []string{"ls"},
		Example: heredoc.Docf(`
			$ %s worker listening-tentacle list
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			dependencies := cmd.NewDependencies(f, c)
			options := list.NewListOptions(dependencies, c, func(worker *machines.Worker) bool {
				return worker.Endpoint.GetCommunicationStyle() == "TentaclePassive"
			})
			return list.ListRun(options)
		},
	}

	return cmd
}
