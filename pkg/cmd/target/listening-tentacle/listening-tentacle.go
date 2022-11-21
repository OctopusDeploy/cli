package listening_tentacle

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdListeningTentacle(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listening-tentacle <command>",
		Short:   "Manage Listening Tentacle deployment targets",
		Long:    "Manage Listening Tentacle deployment targets in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target listening-tentacle list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	return cmd
}
