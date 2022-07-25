package account

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/account/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAccount(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account <command>",
		Short:   "Manage accounts",
		Long:    `Work with Octopus Deploy accounts.`,
		Example: fmt.Sprintf("$ %s account list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	return cmd
}
