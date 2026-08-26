package kubernetes

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdGateway "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway"
	cmdInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/install"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdKubernetes(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubernetes <command>",
		Short: "Manage Kubernetes infrastructure",
		Long: heredoc.Doc(`
			Install and manage the Octopus components that run in a Kubernetes cluster.
		`),
		Example: heredoc.Docf("$ %s kubernetes install", constants.ExecutableName),
		Aliases: []string{"k8s"},
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdInstall.NewCmdInstall(f))
	cmd.AddCommand(cmdGateway.NewCmdGateway(f))

	return cmd
}
