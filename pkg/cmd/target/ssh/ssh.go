package ssh

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh <command>",
		Short:   "Manage SSH deployment targets",
		Long:    "Work with Octopus Deploy ssh deployment targets.",
		Example: fmt.Sprintf("$ %s deployment-target ssh create", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	return cmd
}
