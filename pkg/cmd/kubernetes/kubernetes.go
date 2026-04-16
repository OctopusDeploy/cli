package kubernetes

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdLiveStatus "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/live-status"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdKubernetes(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kubernetes <command>",
		Short:   "Kubernetes observability commands",
		Long:    "Commands for observing Kubernetes resources deployed via Octopus Deploy",
		Example: heredoc.Docf("$ %s kubernetes live-status --project MyProject --environment Production", constants.ExecutableName),
		Aliases: []string{"k8s"},
	}

	cmd.AddCommand(cmdLiveStatus.NewCmdLiveStatus(f))

	return cmd
}
