package azure_web_app

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

func NewCmdAzureWebApp(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "azure-web-app <command>",
		Short:   "Manage Azure Web App deployment targets",
		Long:    "Manage Azure Web App deployment targets in Octopus Deploy",
		Example: fmt.Sprintf("%s deployment-target azure-web-app list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
