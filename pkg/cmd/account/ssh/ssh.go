package ssh

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/ssh/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/ssh/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH Key Pair accounts",
		Long:    "Manage SSH Key Pair accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account ssh list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
