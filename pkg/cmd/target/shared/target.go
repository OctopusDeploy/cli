package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"math"
)

type GetTargetsCallback func() ([]*machines.DeploymentTarget, error)

type GetTargetsOptions struct {
	GetTargetsCallback
}

func NewGetTargetsOptions(dependencies *cmd.Dependencies, query machines.MachinesQuery) *GetTargetsOptions {
	return &GetTargetsOptions{
		GetTargetsCallback: func() ([]*machines.DeploymentTarget, error) {
			return GetAllTargets(*dependencies.Client, query)
		},
	}
}

func NewGetTargetsOptionsForAllTargets(dependencies *cmd.Dependencies) *GetTargetsOptions {
	return &GetTargetsOptions{
		GetTargetsCallback: func() ([]*machines.DeploymentTarget, error) {
			return GetAllTargets(*dependencies.Client, machines.MachinesQuery{})
		},
	}
}

func GetAllTargets(client client.Client, query machines.MachinesQuery) ([]*machines.DeploymentTarget, error) {
	query.Skip = 0
	query.Take = math.MaxInt32
	res, err := client.Machines.Get(query)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}
