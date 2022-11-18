package cloud_region

import (
	"fmt"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdCloudRegion(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cloud-region <command>",
		Short:   "Manage cloud region deployment targets",
		Long:    "Work with Octopus Deploy cloud region deployment targets.",
		Example: fmt.Sprintf("$ %s deployment-target cloud-region list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	return cmd
}
