package polling_tentacle

import (
	"fmt"

	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/polling-tentacle/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPollingTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "polling-tentacle <command>",
		Short:   "Manage polling tentacle deployment targets",
		Long:    "Work with Octopus Deploy polling tentacle deployment targets.",
		Example: fmt.Sprintf("$ %s deployment-target polling-tenatacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	return cmd
}
