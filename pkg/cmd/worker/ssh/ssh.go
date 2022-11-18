package ssh

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH workers",
		Long:    "Work with SSH workers in Octopus Deploy.",
		Example: fmt.Sprintf("$ %s worker SSH list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
