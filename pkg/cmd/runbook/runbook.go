package runbook

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/runbook/list"
	cmdRun "github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdRunbook(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runbook <command>",
		Short: "Manage/run runbooks",
		Long:  `Work with Octopus Deploy runbooks.`,
		Example: heredoc.Docf(`
			$ %s runbook list
			$ %s runbook run
			`, constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdRun.NewCmdRun(f))
	return cmd
}
