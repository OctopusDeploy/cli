package worker

import (
	"github.com/MakeNowJust/heredoc/v2"
	agentInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/agent/install"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

// NewCmdWorker is the same agent as `kubernetes agent`, registered as a worker
// instead of a deployment target. They are separate commands because that is
// the choice being made, and one agent can only be one of the two.
func NewCmdWorker(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker <command>",
		Short: "Manage the Octopus Kubernetes agent running as a worker",
		Long: heredoc.Doc(`
			Manage the Octopus Kubernetes agent running as a worker, which executes steps in a
			Kubernetes cluster and releases the compute again when each task finishes.
		`),
		Example: heredoc.Docf("$ %s kubernetes worker install", constants.ExecutableName),
	}

	cmd.AddCommand(agentInstall.NewCmdWorkerInstall(f))

	return cmd
}
