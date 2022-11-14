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

type GetAllWorkerPoolsCallback func() ([]workerpools.IWorkerPool, error)

type CreateTargetWorkerPoolFlags struct {
	WorkerPool *flag.Flag[string]
}

type CreateTargetWorkerPoolOptions struct {
	*cmd.Dependencies
	GetAllWorkerPoolsCallback
}

func NewCreateTargetWorkerPoolFlags() *CreateTargetWorkerPoolFlags {
	return &CreateTargetWorkerPoolFlags{
		WorkerPool: flag.New[string](FlagWorkerPool, false),
	}
}

func NewCreateTargetWorkerPoolOptions(dependencies *cmd.Dependencies) *CreateTargetWorkerPoolOptions {
	return &CreateTargetWorkerPoolOptions{
		Dependencies: dependencies,
		GetAllWorkerPoolsCallback: func() ([]workerpools.IWorkerPool, error) {
			return getAllWorkerPools(dependencies.Client)
		},
	}
}

func RegisterCreateTargetWorkerPoolFlags(cmd *cobra.Command, flags *CreateTargetWorkerPoolFlags) {
	cmd.Flags().StringVar(&flags.WorkerPool.Value, flags.WorkerPool.Name, "", "The worker pool for the deployment target, only required if not using the default worker pool")
}

func PromptForWorkerPool(opts *CreateTargetWorkerPoolOptions, flags *CreateTargetWorkerPoolFlags) error {
	if flags.WorkerPool.Value == "" {
		useDefaultPool := true
		err := opts.Ask(&survey.Confirm{
			Message: "Will this worker use the default worker pool?",
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
				func(p workerpools.IWorkerPool) string {
					return p.GetName()
				})
			if err != nil {
				return err
			}
			flags.WorkerPool.Value = selectedPool.GetName()
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
		if strings.EqualFold(nameOrId, pool.GetID()) || strings.EqualFold(nameOrId, pool.GetName()) {
			return pool.GetID(), nil
		}
	}

	return "", fmt.Errorf("cannot find worker pool '%s'", nameOrId)
}

func getAllWorkerPools(client *client.Client) ([]workerpools.IWorkerPool, error) {
	res, err := client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
