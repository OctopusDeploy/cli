package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SSH deployment targets",
		Long:  "List SSH deployment targets in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s deployment-target ssh list
			%[1]s deployment-target ssh ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			dependencies, err := cmd.NewDependencies(f, c)
			if err != nil {
				return err
			}
			options := list.NewListOptions(dependencies, c, machines.MachinesQuery{DeploymentTargetTypes: []string{"Ssh"}})
			return list.ListRun(options)
		},
	}

	return cmd
}
