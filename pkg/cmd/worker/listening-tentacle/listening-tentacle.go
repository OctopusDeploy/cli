package listening_tentacle

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdListeningTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listening-tentacle <command>",
		Short:   "Manage Listening Tentacle workers",
		Long:    "Manage Listening Tentacle workers in Octopus Deploy",
		Example: fmt.Sprintf("$ %s worker listening-tentacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
