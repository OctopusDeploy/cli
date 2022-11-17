package ssh

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH workers",
		Long:    "Work with Octopus Deploy SSH workers.",
		Example: fmt.Sprintf("$ %s worker SSH list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
