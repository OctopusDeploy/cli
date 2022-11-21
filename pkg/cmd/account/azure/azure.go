package azure

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAzure(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "azure <command>",
		Short:   "Manage Azure subscription accounts",
		Long:    "Manage Azure subscription accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account azure list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
