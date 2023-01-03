package shared

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"strings"
)

const FlagWorkerPool = "worker-pool"

type GetAllWorkerPoolsCallback func() ([]*workerpools.WorkerPoolListResult, error)

type WorkerPoolFlags struct {
	WorkerPool *flag.Flag[string]
}

type WorkerPoolOptions struct {
	*cmd.Dependencies
	GetAllWorkerPoolsCallback
}

func NewWorkerPoolFlags() *WorkerPoolFlags {
	return &WorkerPoolFlags{
		WorkerPool: flag.New[string](FlagWorkerPool, false),
	}

}

func NewWorkerPoolOptionsForCreateTarget(dependencies *cmd.Dependencies) *WorkerPoolOptions {
	return &WorkerPoolOptions{
		Dependencies: dependencies,
		GetAllWorkerPoolsCallback: func() ([]*workerpools.WorkerPoolListResult, error) {
			return getAllWorkerPools(dependencies.Client)
		},
	}
}

func RegisterCreateTargetWorkerPoolFlags(cmd *cobra.Command, flags *WorkerPoolFlags) {
	cmd.Flags().StringVar(&flags.WorkerPool.Value, flags.WorkerPool.Name, "", "The worker pool for the deployment target, only required if not using the default worker pool")
}

func PromptForWorkerPool(opts *WorkerPoolOptions, flags *WorkerPoolFlags) error {
	if flags.WorkerPool.Value == "" {
		useDefaultPool := true
		err := opts.Ask(&survey.Confirm{
			Message: "Will this target use the default worker pool?",
			Default: true,
		}, &useDefaultPool)
		if err != nil {
			return err
		}
		if !useDefaultPool {
			selectedPool, err := selectors.Select(
				opts.Ask,
				"Select the worker pool to use",
				opts.GetAllWorkerPoolsCallback,
				func(p *workerpools.WorkerPoolListResult) string {
					return p.Name
				})
			if err != nil {
				return err
			}
			flags.WorkerPool.Value = selectedPool.Name
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func FindWorkerPoolId(getAllWorkerPools GetAllWorkerPoolsCallback, nameOrId string) (string, error) {
	pools, err := getAllWorkerPools()
	if err != nil {
		return "", err
	}

	for _, p := range pools {
		pool := p
		if strings.EqualFold(nameOrId, pool.ID) || strings.EqualFold(nameOrId, pool.Name) {
			return pool.ID, nil
		}
	}

	return "", fmt.Errorf("cannot find worker pool '%s'", nameOrId)
}

func getAllWorkerPools(client *client.Client) ([]*workerpools.WorkerPoolListResult, error) {
	res, err := client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
