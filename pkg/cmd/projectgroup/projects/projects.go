package groupprojects

import (
	"fmt"

	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/projects/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdListProjects(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects <command>",
		Short:   "List all projects",
		Long:    "List all projects in a group in Octopus Deploy",
		Example: fmt.Sprintf("$ %s project-group projects list --group <GroupName>", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
