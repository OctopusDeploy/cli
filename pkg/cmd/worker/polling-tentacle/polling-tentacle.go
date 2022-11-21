package polling_tentacle

import (
	"fmt"

	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/polling-tentacle/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/worker/polling-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPollingTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "polling-tentacle <command>",
		Short:   "Manage Polling Tentacle workers",
		Long:    "Manage Polling Tentacle workers in Octopus Deploy",
		Example: fmt.Sprintf("$ %s worker polling-tentacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
