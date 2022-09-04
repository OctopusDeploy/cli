package fixtures

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
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
	result.SpaceID = spaceID
	result.ID = "deploymentprocess-" + projectID
	result.Links = map[string]string{
		"Template": fmt.Sprintf("/api/%s/projects/%s/deploymentprocesses/template{?channel,releaseId}", spaceID, projectID),
	}
	return result
}

func NewDeploymentProcessForVersionControlledProject(spaceID string, projectID string, gitRef string) *deployments.DeploymentProcess {
	result := deployments.NewDeploymentProcess(projectID)
	result.SpaceID = spaceID
	result.ID = "deploymentprocess-" + projectID
	result.Links = map[string]string{
		"Template": fmt.Sprintf("/api/%s/projects/%s/%s/deploymentprocesses/template{?channel,releaseId}", spaceID, projectID, gitRef),
	}
	return result
}

func NewDeploymentSettingsForProject(spaceID string, projectID string, versioningStrategy *projects.VersioningStrategy) *deployments.DeploymentSettings {
	result := deployments.NewDeploymentSettings()
	result.SpaceID = spaceID
	result.ProjectID = projectID
	result.ID = "deploymentsettings-" + projectID
	result.VersioningStrategy = versioningStrategy
	// DeploymentSettings just has links to self and project, which aren't particularly useful here
	return result
}

// NewProject creates a new project resource, using default settings from the server.
// NOT tenanted
// VersioningStrategy is template based
// NOT version controlled
func NewProject(spaceID string, projectID string, projectName string, lifecycleID string, projectGroupID string, deploymentProcessID string) *projects.Project {
	result := projects.NewProject(projectName, lifecycleID, projectGroupID)
	result.ID = projectID
	result.VersioningStrategy = &projects.VersioningStrategy{
		Template: "#{Octopus.Version.LastMajor}.#{Octopus.Version.LastMinor}.#{Octopus.Version.NextPatch}", // this is the default
	}
	result.PersistenceSettings = projects.NewDatabasePersistenceSettings()
	result.DeploymentProcessID = deploymentProcessID
	result.TenantedDeploymentMode = core.TenantedDeploymentModeUntenanted
	result.Links = map[string]string{
		"Channels":           fmt.Sprintf("/api/%s/projects/%s/channels{/id}{?skip,take,partialName}", spaceID, projectID),
		"DeploymentProcess":  fmt.Sprintf("/api/%s/projects/%s/deploymentprocesses", spaceID, projectID),
		"DeploymentSettings": fmt.Sprintf("/api/%s/projects/%s/deploymentsettings", spaceID, projectID),
		"Releases":           fmt.Sprintf("/api/%s/projects/%s/releases{/version}{?skip,take,searchByVersion}", spaceID, projectID),
	}
	return result
}

func NewVersionControlledProject(spaceID string, projectID string, projectName string, lifecycleID string, projectGroupID string, deploymentProcessID string) *projects.Project {
	repoUrl, _ := url.Parse("https://server/repo.git")

	result := NewProject(spaceID, projectID, projectName, lifecycleID, projectGroupID, deploymentProcessID)
	result.VersioningStrategy = nil // CaC projects seem to always report nil here via the API
	result.PersistenceSettings = projects.NewGitPersistenceSettings(".octopus", projects.NewAnonymousGitCredential(), "main", repoUrl)

	// CaC projects have different values in these links
	result.Links["DeploymentProcess"] = fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentprocesses", spaceID, projectID) // note gitRef is a template param in the middle of the url path
	result.Links["DeploymentSettings"] = fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentsettings", spaceID, projectID) // note gitRef is a template param in the middle of the url path

	// CaC projects have extra links
	result.Links["Tags"] = fmt.Sprintf("/api/%s/projects/%s/git/tags{/name}{?skip,take,searchByName,refresh}", spaceID, projectID)
	result.Links["Branches"] = fmt.Sprintf("/api/%s/projects/%s/git/branches{/name}{?skip,take,searchByName,refresh}", spaceID, projectID)
	result.Links["Commits"] = fmt.Sprintf("/api/%s/projects/%s/git/commits{/hash}{?skip,take,refresh}", spaceID, projectID)
	return result
}

func NewChannel(spaceID string, channelID string, channelName string, projectID string) *channels.Channel {
	result := channels.NewChannel(channelName, projectID)
	result.ID = channelID
	result.SpaceID = spaceID
	return result
}

func NewRelease(spaceID string, releaseID string, releaseVersion string, projectID string, channelID string) *releases.Release {
	result := releases.NewRelease(channelID, projectID, releaseVersion)
	result.ID = releaseID
	result.SpaceID = spaceID
	return result
}

func NewEnvironment(spaceID string, envID string, name string) *environments.Environment {
	result := environments.NewEnvironment(name)
	result.ID = envID
	result.SpaceID = spaceID
	return result
}

func NewVariableSetForProject(spaceID string, projectID string) *variables.VariableSet {
	result := variables.NewVariableSet()
	result.OwnerID = projectID
	result.SpaceID = spaceID
	result.Variables = make([]*variables.Variable, 0)
	result.ID = "variableset-" + projectID
	result.Links = map[string]string{}
	return result
}
