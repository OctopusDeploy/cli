package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"math"
)

type GetRunbooksCallback func(projectID string) ([]*runbooks.Runbook, error)
type GetAllProjectsCallback func() ([]*projects.Project, error)

type RunbooksOptions struct {
	GetRunbooksCallback
	GetAllProjectsCallback
}

func NewGetRunbooksOptions(dependencies *cmd.Dependencies) *RunbooksOptions {
	return &RunbooksOptions{
		GetRunbooksCallback: func(projectID string) ([]*runbooks.Runbook, error) {
			return GetAllRunbooks(dependencies.Client, projectID)
		},
	}
}

func GetAllRunbooks(client newclient.Client, projectID string) ([]*runbooks.Runbook, error) {
	res, err := runbooks.List(client, client.GetSpaceID(), projectID, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func GetAllProjects(client *client.Client) ([]*projects.Project, error) {
	res, err := client.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
