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
		Short:   "Manage listening tentacle deployment targets",
		Long:    "Work with Octopus Deploy listening tentacle deployment targets.",
		Example: fmt.Sprintf("$ %s deployment-target listening-tenatacle list", constants.ExecutableName),
	}

	//cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	return cmd
}
