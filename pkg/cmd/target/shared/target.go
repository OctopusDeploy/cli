package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type GetAllTargetsCallback func() ([]*machines.DeploymentTarget, error)

type GetAllTargetsOptions struct {
	GetAllTargetsCallback
}

func NewGetAllTargetsOptions(dependencies *cmd.Dependencies) *GetAllTargetsOptions {
	return &GetAllTargetsOptions{
		GetAllTargetsCallback: func() ([]*machines.DeploymentTarget, error) {
			return getAllTargets(*dependencies.Client)
		},
	}
}

func getAllTargets(client client.Client) ([]*machines.DeploymentTarget, error) {
	res, err := client.Machines.GetAll()
	if err != nil {
		return nil, err
	}
	return res, nil
}
