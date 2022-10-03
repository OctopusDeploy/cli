package runbook

import (
	"fmt"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/runbook/list"
	cmdListRuns "github.com/OctopusDeploy/cli/pkg/cmd/runbook/list-runs"
	cmdRun "github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdRunbook(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runbook <command>",
		Short:   "Manage runbooks",
		Long:    `Work with Octopus Deploy runbooks.`,
		Example: fmt.Sprintf("$ %s runbook list", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdRun.NewCmdRun(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdListRuns.NewCmdListRuns(f))

	return cmd
}
