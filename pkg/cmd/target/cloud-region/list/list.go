package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Cloud Region deployment targets in an instance of Octopus Deploy",
		Long:  "List Cloud Region deployment targets in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target cloud-region list
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			dependencies := cmd.NewDependencies(f, c)
			options := list.NewListOptions(dependencies, c, machines.MachinesQuery{DeploymentTargetTypes: []string{"CloudRegion"}})
			return list.ListRun(options)
		},
	}

	return cmd
}
