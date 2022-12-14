package kubernetes

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/target/kubernetes/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/target/kubernetes/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdKubernetes(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kubernetes <command>",
		Short:   "Manage Kubernetes deployment targets",
		Long:    "Manage Kubernetes deployment targets in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target Kubernetes create", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
