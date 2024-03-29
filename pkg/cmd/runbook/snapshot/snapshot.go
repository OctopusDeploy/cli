package snapshot

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/runbook/snapshot/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSnapshot(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot <command>",
		Short: "Manage runbook snapshots",
		Long:  "Manage runbook snapshots in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot create
			$ %[1]s runbook snapshot list
		`, constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	return cmd
}
