package view

import (
	"fmt"
	dynamicPool "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/dynamic/view"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	staticPool "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/static/view"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a worker pool",
		Long:  "View a worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker-pool view WorkerPools-3
			$ %[1]s worker-pool view 'linux workers'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var workerPool, err = opts.Client.WorkerPools.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	switch workerPool.GetWorkerPoolType() {
	case workerpools.WorkerPoolTypeDynamic:
		return dynamicPool.ViewRun(opts)
	case workerpools.WorkerPoolTypeStatic:
		return staticPool.ViewRun(opts)
	}

	return fmt.Errorf("unsupported worker pool '%s'", workerPool.GetWorkerPoolType())
}
