package target

import (
	"fmt"
	cmdCloudRegion "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/target/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/list"
	cmdListeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdDeploymentTarget(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deployment-target <command>",
		Short:   "Manage deployment targets",
		Long:    `Work with Octopus Deploy deployment targets.`,
		Example: fmt.Sprintf("$ %s deployment-target list", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdListeningTentacle.NewCmdListeningTentacle(f))
	cmd.AddCommand(cmdCloudRegion.NewCmdCloudRegion(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
