package view

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a dynamic worker pool",
		Long:  "View a dynamic worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker-pool dynamic view WorkerPools-3
			$ %[1]s worker-pool dynamic view 'Hosted Workers'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args, c))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeDetails)
}

func contributeDetails(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) ([]*output.DataRow, error) {
	workerType := workerPool.(*workerpools.DynamicWorkerPool).WorkerType

	data := []*output.DataRow{}
	data = append(data, output.NewDataRow("Worker Type", string(workerType)))

	return data, nil

}
