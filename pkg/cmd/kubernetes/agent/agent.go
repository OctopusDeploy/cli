package agent

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/agent/install"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAgent(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent <command>",
		Short: "Manage the Octopus Kubernetes agent",
		Long: heredoc.Doc(`
			Manage the Octopus Kubernetes agent, which runs Kubernetes deployments from inside
			the cluster they target.
		`),
		Example: heredoc.Docf("$ %s kubernetes agent install", constants.ExecutableName),
	}

	cmd.AddCommand(cmdInstall.NewCmdInstall(f))

	return cmd
}
