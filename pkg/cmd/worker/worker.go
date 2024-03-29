package worker

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/worker/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/list"
	listeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle"
	pollingTentacle "github.com/OctopusDeploy/cli/pkg/cmd/worker/polling-tentacle"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/worker/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdWorker(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker <command>",
		Short: "Manage workers",
		Long:  "Manage workers in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker list
			$ %[1]s worker ls
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(listeningTentacle.NewCmdListeningTentacle(f))
	cmd.AddCommand(pollingTentacle.NewCmdPollingTentacle(f))
	cmd.AddCommand(ssh.NewCmdSsh(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
