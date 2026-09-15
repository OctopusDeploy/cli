package disable

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/spf13/cobra"
)

func NewCmdDisable(f factory.Factory) *cobra.Command {
	return &cobra.Command{
		Args:  usage.MaximumNArgs(1),
		Use:   "disable [<name> | <id>]",
		Short: "Disable a worker",
		Long:  "Disable a worker in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s worker disable Workers-100
			%[1]s worker disable 'build-worker'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			dependencies := cmd.NewDependencies(f, c)
			opts := machinescommon.NewSetDisabledStateOptions(args, dependencies, machinescommon.NewWorkerKind(dependencies), true)
			return machinescommon.SetDisabledState(opts)
		},
	}
}
