package listening_tentacle

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/create"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdListeningTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listening-tentacle <command>",
		Short:   "Manage listening tentacle deployment targets",
		Long:    "Work with Octopus Deploy listening tentacle deployment targets.",
		Example: fmt.Sprintf("$ %s deployment-target listening-tenatacle list", constants.ExecutableName),
	}

	//cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	return cmd
}
