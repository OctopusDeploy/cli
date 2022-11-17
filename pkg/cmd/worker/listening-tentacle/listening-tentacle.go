package listening_tentacle

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdListeningTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listening-tentacle <command>",
		Short:   "Manage listening tentacle workers",
		Long:    "Work with Octopus Deploy listening tentacle workers.",
		Example: fmt.Sprintf("$ %s worker listening-tentacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
