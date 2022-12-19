package list

import (
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
		Short: "List Kubernetes deployment targets",
		Long:  "List Kubernetes deployment targets in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s deployment-target kubernetes list
			$ %[1]s deployment-target kubernetes ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, _ []string) error {
			dependencies := cmd.NewDependencies(f, c)
			options := list.NewListOptions(dependencies, c, machines.MachinesQuery{DeploymentTargetTypes: []string{"Kubernetes"}})
			return list.ListRun(options)
		},
	}

	return cmd
}
