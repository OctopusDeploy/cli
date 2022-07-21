package account

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/account/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdAccount(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account <command>",
		Short:   "Manage accounts",
		Long:    `Work with Octopus Deploy accounts.`,
		Example: fmt.Sprintf("$ %s account list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdDelete.NewCmdDelete(client))
	cmd.AddCommand(cmdCreate.NewCmdCreate(client))
	cmd.AddCommand(cmdList.NewCmdList(client))
	return cmd
}
