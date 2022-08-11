package fixtures

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"net/url"
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
	result.VersioningStrategy = &projects.VersioningStrategy{
		Template: "#{Octopus.Version.LastMajor}.#{Octopus.Version.LastMinor}.#{Octopus.Version.NextPatch}", // this is the default
	}
	result.PersistenceSettings = projects.NewDatabasePersistenceSettings()
	result.DeploymentProcessID = deploymentProcessID
	result.Links = map[string]string{
		"Channels":           fmt.Sprintf("/api/%s/projects/%s/channels{/id}{?skip,take,partialName}", spaceID, projectID),
		"DeploymentProcess":  fmt.Sprintf("/api/%s/projects/%s/deploymentprocesses", spaceID, projectID),
		"DeploymentSettings": fmt.Sprintf("/api/%s/projects/%s/deploymentsettings", spaceID, projectID),
	}
	return result
}

func NewVersionControlledProject(spaceID string, projectID string, projectName string, lifecycleID string, projectGroupID string, deploymentProcessID string) *projects.Project {
	repoUrl, _ := url.Parse("https://server/repo.git")

	result := projects.NewProject(projectName, lifecycleID, projectGroupID)
	result.ID = projectID
	result.VersioningStrategy = nil // CaC projects seem to always report nil here via the API
	result.PersistenceSettings = projects.NewGitPersistenceSettings(".octopus", projects.NewAnonymousGitCredential(), "main", repoUrl)
	result.DeploymentProcessID = deploymentProcessID
	result.Links = map[string]string{
		"Channels":           fmt.Sprintf("/api/%s/projects/%s/channels{/id}{?skip,take,partialName}", spaceID, projectID),
		"DeploymentProcess":  fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentprocesses", spaceID, projectID), // note gitRef is a template param in the middle of the url path
		"DeploymentSettings": fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentsettings", spaceID, projectID),  // note gitRef is a template param in the middle of the url path
		"Tags":               fmt.Sprintf("/api/%s/projects/%s/git/tags{/name}{?skip,take,searchByName,refresh}", spaceID, projectID),
		"Branches":           fmt.Sprintf("/api/%s/projects/%s/git/branches{/name}{?skip,take,searchByName,refresh}", spaceID, projectID),
		"Commits":            fmt.Sprintf("/api/%s/projects/%s/git/commits{/hash}{?skip,take,refresh}", spaceID, projectID),
	}
	return result
}

func NewChannel(spaceID string, channelID string, channelName string, projectID string) *channels.Channel {
	result := channels.NewChannel(channelName, projectID)
	result.ID = channelID
	result.SpaceID = spaceID
	return result
}
