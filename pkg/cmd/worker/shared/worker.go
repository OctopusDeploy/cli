package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type GetWorkersCallback func() ([]*machines.Worker, error)

type GetWorkersOptions struct {
	GetWorkersCallback
}

func NewGetWorkersOptions(dependencies *cmd.Dependencies, filter func(*machines.Worker) bool) *GetWorkersOptions {
	return &GetWorkersOptions{
		GetWorkersCallback: func() ([]*machines.Worker, error) {
			return GetWorkers(*dependencies.Client, filter)
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
