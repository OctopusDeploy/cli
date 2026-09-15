package enable

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/spf13/cobra"
)

func NewCmdEnable(f factory.Factory) *cobra.Command {
	return &cobra.Command{
		Args:  usage.MaximumNArgs(1),
		Use:   "enable [<name> | <id>]",
		Short: "Enable a deployment target",
		Long:  "Enable a deployment target in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s deployment-target enable Machines-100
			%[1]s deployment-target enable 'web-server'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			dependencies := cmd.NewDependencies(f, c)
			opts := machinescommon.NewSetDisabledStateOptions(args, dependencies, machinescommon.NewDeploymentTargetKind(dependencies), false)
			return machinescommon.SetDisabledState(opts)
		},
	}
}
