package view

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a Polling Tentacle worker",
		Long:  "View a Polling Tentacle worker in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker polling-tentacle view 'WindowsWorker'
			$ %[1]s worker polling-tentacle view Machines-100
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args, c)
			return ViewRun(opts)
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeEndpoint, "Polling Tentacle")
}

func contributeEndpoint(opts *shared.ViewOptions, workerEndpoint machines.IEndpoint) ([]*output.DataRow, error) {
	data := []*output.DataRow{}

	endpoint := workerEndpoint.(*machines.PollingTentacleEndpoint)
	data = append(data, output.NewDataRow("URI", endpoint.URI.String()))
	data = append(data, output.NewDataRow("Tentacle version", endpoint.TentacleVersionDetails.Version))

	return data, nil
}
