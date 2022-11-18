package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

const FlagWorkerPool = "worker-pool"

type GetAllWorkerPoolsCallback func() ([]*workerpools.WorkerPoolListResult, error)

type WorkerPoolFlags struct {
	WorkerPools *flag.Flag[[]string]
}

type WorkerPoolOptions struct {
	*cmd.Dependencies
	GetAllWorkerPoolsCallback
}

func NewWorkerPoolFlags() *WorkerPoolFlags {
	return &WorkerPoolFlags{
		WorkerPools: flag.New[[]string](FlagWorkerPool, false),
	}
}

func RegisterCreateWorkerWorkerPoolFlags(cmd *cobra.Command, flags *WorkerPoolFlags) {
	cmd.Flags().StringSliceVar(&flags.WorkerPools.Value, flags.WorkerPools.Name, []string{}, "The worker pools which the worker will be a member of")
}

func NewWorkerPoolOptions(dependencies *cmd.Dependencies) *WorkerPoolOptions {
	return &WorkerPoolOptions{
		Dependencies: dependencies,
		GetAllWorkerPoolsCallback: func() ([]*workerpools.WorkerPoolListResult, error) {
			return getAllWorkerPools(dependencies.Client)
		},
	}
}

func getAllWorkerPools(client *client.Client) ([]*workerpools.WorkerPoolListResult, error) {
	res, err := client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}

	return util.SliceFilter(res, func(workerPool *workerpools.WorkerPoolListResult) bool {
		return workerPool.CanAddWorkers
	}), nil
}

func FindWorkerPoolIds(opts *WorkerPoolOptions, flags *WorkerPoolFlags) ([]string, error) {
	var ids []string

	lookup := make(map[string]string)
	for _, i := range flags.WorkerPools.Value {
		lookup[i] = i
	}

	allWorkerPools, err := opts.GetAllWorkerPoolsCallback()
	if err != nil {
		return nil, err
	}
	for _, p := range allWorkerPools {
		if _, ok := lookup[p.ID]; ok {
			ids = append(ids, p.ID)
		} else if _, ok := lookup[p.Name]; ok {
			ids = append(ids, p.ID)
		}
	}

	return ids, nil
}

func PromptForWorkerPools(opts *WorkerPoolOptions, flags *WorkerPoolFlags) error {
	if util.Empty(flags.WorkerPools.Value) {
		allWorkerPools, err := opts.GetAllWorkerPoolsCallback()
		if err != nil {
			return err
		}
		selectedPools, err := question.MultiSelectMap(opts.Ask, "Select the worker pools to assign to the worker", allWorkerPools, func(pool *workerpools.WorkerPoolListResult) string { return pool.Name }, true)
		if err != nil {
			return err
		}

		for _, p := range selectedPools {
			flags.WorkerPools.Value = append(flags.WorkerPools.Value, p.Name)
		}
	}

	return nil
}
