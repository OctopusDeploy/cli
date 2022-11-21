package ssh

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH deployment targets",
		Long:    "Manage SSH deployment targets in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target ssh create", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	return cmd
}
