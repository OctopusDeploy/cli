package worker

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/worker/list"
	listeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdWorker(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker <command>",
		Short: "Manage workers",
		Long:  `Work with Octopus Deploy workers.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s worker list
			$ %s tenant ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(listeningTentacle.NewCmdListeningTentacle(f))
	cmd.AddCommand(ssh.NewCmdSsh(f))
	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
