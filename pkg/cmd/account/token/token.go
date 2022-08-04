package aws

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/token/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/token/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdToken(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "token <command>",
		Short:   "Manage Token accounts",
		Long:    `Work with Octopus Deploy Token accounts.`,
		Example: fmt.Sprintf("$ %s account token list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
