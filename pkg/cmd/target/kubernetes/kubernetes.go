package kubernetes

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/target/kubernetes/create"
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
		Example: heredoc.Docf("$ %s deployment-target kubernetes create", constants.ExecutableName),
		Aliases: []string{"k8s"},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
