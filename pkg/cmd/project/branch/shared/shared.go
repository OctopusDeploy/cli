package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectbranches"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

type GetAllBranchesCallback func(projectId string) ([]*projects.GitReference, error)

type ProjectBranchCallbacks struct {
	GetAllBranchesCallback GetAllBranchesCallback
}

func NewProjectBranchCallbacks(dependencies *cmd.Dependencies) *ProjectBranchCallbacks {
	return &ProjectBranchCallbacks{
		GetAllBranchesCallback: func(projectId string) ([]*projects.GitReference, error) {
			return getAllBranches(dependencies.Client, dependencies.Space.GetID(), projectId)
		},
	}
}

func getAllBranches(client *client.Client, spaceId string, projectId string) ([]*projects.GitReference, error) {
	branches, err := client.ProjectBranches.Get(spaceId, projectId, projectbranches.ProjectBranchQuery{Skip: 0, Take: 9999})
	if err != nil {
		return nil, err
	}
	return branches.Items, nil
}
