package azure

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAzure(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "azure <command>",
		Short:   "Manage Azure accounts",
		Long:    `Work with Octopus Deploy Azure subscription accounts.`,
		Example: fmt.Sprintf("$ %s account azure list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
