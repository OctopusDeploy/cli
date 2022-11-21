package username

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/username/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/username/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdUsername(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "username <command>",
		Short:   "Manage Username/Password accounts",
		Long:    "Manage Username/Password accounts in Octopus Deploy",
		Example: fmt.Sprintf("$ %s account username list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
