package polling_tentacle

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/polling-tentacle/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/polling-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPollingTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "polling-tentacle <command>",
		Short:   "Manage Polling Tentacle deployment targets",
		Long:    "Manage Polling Tentacle deployment targets in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target polling-tenatacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	return cmd
}
