package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
)

type GetWorkerPoolsCallback func() ([]*workerpools.WorkerPoolListResult, error)
type GetWorkerPoolCallback func(identifer string) (workerpools.IWorkerPool, error)

type GetWorkerPoolsOptions struct {
	GetWorkerPoolsCallback
	GetWorkerPoolCallback
}

func NewGetWorkerPoolsOptions(dependencies *cmd.Dependencies) *GetWorkerPoolsOptions {
	return &GetWorkerPoolsOptions{
		GetWorkerPoolsCallback: func() ([]*workerpools.WorkerPoolListResult, error) {
			return GetWorkerPools(*dependencies.Client)
		},
		GetWorkerPoolCallback: func(identifier string) (workerpools.IWorkerPool, error) {
			return GetWorker(*dependencies.Client, identifier)
		},
	}
}

func GetWorkerPools(client client.Client) ([]*workerpools.WorkerPoolListResult, error) {
	allWorkerPools, err := client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}

	return allWorkerPools, nil
}

func GetWorker(client client.Client, identifier string) (workerpools.IWorkerPool, error) {
	workerPool, err := client.WorkerPools.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return workerPool, nil
}
