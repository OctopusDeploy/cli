package ssh

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH workers",
		Long:    "Manage SSH workers in Octopus Deploy",
		Example: heredoc.Docf("$ %s worker SSH list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
