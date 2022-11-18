package list

import (
	"fmt"
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
		Short:   "List Listening Tentacle workers in an instance of Octopus Deploy",
		Long:    "List Listening Tentacle workers in an instance of Octopus Deploy.",
		Aliases: []string{"ls"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s worker listening-tentacle list
		`), constants.ExecutableName),
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
