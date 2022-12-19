package target

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdAzureWebApp "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app"
	cmdCloudRegion "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/target/delete"
	cmdKubernetes "github.com/OctopusDeploy/cli/pkg/cmd/target/kubernetes"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/list"
	cmdListeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle"
	cmdPollingTentacle "github.com/OctopusDeploy/cli/pkg/cmd/target/polling-tentacle"
	cmdSsh "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdDeploymentTarget(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deployment-target <command>",
		Short:   "Manage deployment targets",
		Long:    "Manage deployment targets in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target list", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdListeningTentacle.NewCmdListeningTentacle(f))
	cmd.AddCommand(cmdPollingTentacle.NewCmdPollingTentacle(f))
	cmd.AddCommand(cmdSsh.NewCmdSsh(f))
	cmd.AddCommand(cmdCloudRegion.NewCmdCloudRegion(f))
	cmd.AddCommand(cmdAzureWebApp.NewCmdAzureWebApp(f))
	cmd.AddCommand(cmdKubernetes.NewCmdKubernetes(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
