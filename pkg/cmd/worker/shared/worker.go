package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type GetWorkersCallback func() ([]*machines.Worker, error)
type GetWorkerCallback func(identifer string) (*machines.Worker, error)

type GetWorkersOptions struct {
	GetWorkersCallback
	GetWorkerCallback
}

func NewGetWorkersOptions(dependencies *cmd.Dependencies, filter func(*machines.Worker) bool) *GetWorkersOptions {
	return &GetWorkersOptions{
		GetWorkersCallback: func() ([]*machines.Worker, error) {
			return GetWorkers(*dependencies.Client, filter)
		},
		GetWorkerCallback: func(identifier string) (*machines.Worker, error) {
			return GetWorker(*dependencies.Client, identifier)
		},
	}
}

func NewGetWorkersOptionsForAllWorkers(dependencies *cmd.Dependencies) *GetWorkersOptions {
	return &GetWorkersOptions{
		GetWorkersCallback: func() ([]*machines.Worker, error) {
			return GetWorkers(*dependencies.Client, nil)
		},
	}
}

func GetWorkers(client client.Client, filter func(*machines.Worker) bool) ([]*machines.Worker, error) {
	allWorkers, err := client.Workers.GetAll()
	if err != nil {
		return nil, err
	}

	if filter == nil {
		return allWorkers, nil
	}

	var workers []*machines.Worker
	for _, w := range allWorkers {
		filterResult := filter(w)
		if filterResult {
			workers = append(workers, w)
		}
	}

	return workers, nil
}

func GetWorker(client client.Client, identifier string) (*machines.Worker, error) {
	worker, err := client.Workers.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return worker, nil
}
