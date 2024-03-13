package runbook

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/runbook/list"
	cmdRun "github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	cmdSnapshot "github.com/OctopusDeploy/cli/pkg/cmd/runbook/snapshot"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdRunbook(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runbook <command>",
		Short: "Manage runbooks",
		Long:  "Manage runbooks in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook list
			$ %[1]s runbook run
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdRun.NewCmdRun(f))
	cmd.AddCommand(cmdSnapshot.NewCmdSnapshot(f))
	return cmd
}
