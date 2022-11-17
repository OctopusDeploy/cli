package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"math"
)

type GetWorkersCallback func() ([]*machines.Worker, error)

type GetWorkersOptions struct {
	GetWorkersCallback
}

func NewGetWorkersOptions(dependencies *cmd.Dependencies, query machines.WorkersQuery) *GetWorkersOptions {
	return &GetWorkersOptions{
		GetWorkersCallback: func() ([]*machines.Worker, error) {
			return GetAllWorkers(*dependencies.Client, query)
		},
	}
}

func NewGetWorkersOptionsForAllWorkers(dependencies *cmd.Dependencies) *GetWorkersOptions {
	return &GetWorkersOptions{
		GetWorkersCallback: func() ([]*machines.Worker, error) {
			return GetAllWorkers(*dependencies.Client, machines.WorkersQuery{})
		},
	}
}

func GetAllWorkers(client client.Client, query machines.WorkersQuery) ([]*machines.Worker, error) {
	query.Skip = 0
	query.Take = math.MaxInt32
	res, err := client.Workers.Get(query)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}
