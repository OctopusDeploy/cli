package account

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
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

	cmd.AddCommand(cmdList.NewCmdList(client))
	return cmd
}
