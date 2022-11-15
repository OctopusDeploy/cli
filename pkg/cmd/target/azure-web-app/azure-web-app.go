package azure_web_app

import (
	"fmt"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAzureWebApp(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "azure-web-app <command>",
		Short:   "Manage Azure Web App deployment targets",
		Long:    "Work with Azure Web App deployment targets in Octopus Deploy.",
		Example: fmt.Sprintf("$ %s deployment-target listening-tenatacle list", constants.ExecutableName),
	}

	//cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	return cmd
}
