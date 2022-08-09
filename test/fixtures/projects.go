package fixtures

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

// This file contains utility functions for creating mock objects used in unit tests.
// Please try not to put any actual logic in here, that should go in testutil.

func NewSpace(spaceID string, name string) *spaces.Space {
	result := spaces.NewSpace(name)
	result.ID = spaceID
	return result
}

func NewDeploymentProcessForProject(spaceID string, projectID string) *deployments.DeploymentProcess {
	result := deployments.NewDeploymentProcess(projectID)
	result.ID = "deploymentprocess-" + projectID
	result.Links = map[string]string{
		"Template": fmt.Sprintf("/api/%s/projects/%s/deploymentprocesses/template", spaceID, projectID),
	}
	return result
}

func NewProject(spaceID string, projectID string, projectName string, lifecycleID string, projectGroupID string, deploymentProcessID string) *projects.Project {
	result := projects.NewProject(projectName, lifecycleID, projectGroupID)
	result.ID = projectID
	result.PersistenceSettings = projects.NewDatabasePersistenceSettings()
	result.DeploymentProcessID = deploymentProcessID
	result.Links = map[string]string{
		"Channels":          fmt.Sprintf("/api/%s/projects/%s/channels{/id}{?skip,take,partialName}", spaceID, projectID),
		"DeploymentProcess": fmt.Sprintf("/api/%s/projects/%s/deploymentprocesses", spaceID, projectID),
	}
	return result
}
