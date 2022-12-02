package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a static worker pool",
		Long:  "View a static worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker-pool static view WorkerPools-3
			$ %[1]s worker-pool static view 'windows workers'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeDetails)
}

func contributeDetails(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) ([]*output.DataRow, error) {
	workers, err := opts.Client.WorkerPools.GetWorkers(workerPool)
	if err != nil {
		return nil, err
	}

	data := []*output.DataRow{}
	data = append(data, output.NewDataRow("Total workers", fmt.Sprintf("%d", len(workers))))
	data = append(data, output.NewDataRow("Disabled workers", output.Dimf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.IsDisabled })))))
	data = append(data, output.NewDataRow("Healthy workers", output.Greenf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.HealthStatus == "Healthy" })))))
	data = append(data, output.NewDataRow("Has Warnings workers", output.Yellowf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.HealthStatus == "HasWarnings" })))))
	data = append(data, output.NewDataRow("Unhealthy workers", output.Redf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.HealthStatus == "Unhealthy" })))))
	data = append(data, output.NewDataRow("Unavailable workers", output.Redf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.HealthStatus == "Unavailable" })))))
	data = append(data, output.NewDataRow("SSH workers", fmt.Sprintf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.Endpoint.GetCommunicationStyle() == "Ssh" })))))
	data = append(data, output.NewDataRow("Listening Tentacle workers", fmt.Sprintf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.Endpoint.GetCommunicationStyle() == "TentaclePassive" })))))
	data = append(data, output.NewDataRow("Polling Tentacle workers", fmt.Sprintf("%d", len(util.SliceFilter(workers, func(w *machines.Worker) bool { return w.Endpoint.GetCommunicationStyle() == "TentacleActive" })))))

	return data, nil

}
