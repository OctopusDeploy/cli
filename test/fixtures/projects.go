package fixtures

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
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

	protectedBranchNamePatterns := []string{}
	result := NewProject(spaceID, projectID, projectName, lifecycleID, projectGroupID, deploymentProcessID)
	result.VersioningStrategy = nil // CaC projects seem to always report nil here via the API
	result.PersistenceSettings = projects.NewGitPersistenceSettings(".octopus", credentials.NewAnonymous(), "main", protectedBranchNamePatterns, repoUrl)

	// CaC projects have different values in these links
	result.Links["DeploymentProcess"] = fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentprocesses", spaceID, projectID) // note gitRef is a template param in the middle of the url path
	result.Links["DeploymentSettings"] = fmt.Sprintf("/api/%s/projects/%s/{gitRef}/deploymentsettings", spaceID, projectID) // note gitRef is a template param in the middle of the url path

	// CaC projects have extra links
	result.Links["Tags"] = fmt.Sprintf("/api/%s/projects/%s/git/tags{/name}{?skip,take,searchByName,refresh}", spaceID, projectID)
	result.Links["Branches"] = fmt.Sprintf("/api/%s/projects/%s/git/branches{/name}{?skip,take,searchByName,refresh}", spaceID, projectID)
	result.Links["Commits"] = fmt.Sprintf("/api/%s/projects/%s/git/commits{/hash}{?skip,take,refresh}", spaceID, projectID)
	return result
}

func AsServerResponse(project *projects.Project) []byte {
	projectJSON, err := json.Marshal(project)
	if err != nil {
		panic(err)
	}

	if gitSettings, ok := project.PersistenceSettings.(projects.GitPersistenceSettings); ok {
		var projectMap map[string]interface{}
		if err := json.Unmarshal(projectJSON, &projectMap); err != nil {
			panic(err)
		}

		if persistenceSettings, ok := projectMap["PersistenceSettings"].(map[string]interface{}); ok {
			persistenceSettings["ConversionState"] = map[string]interface{}{
				"VariablesAreInGit": gitSettings.VariablesAreInGit(),
				"RunbooksAreInGit":  gitSettings.RunbooksAreInGit(),
			}
		}

		modifiedJSON, err := json.Marshal(projectMap)
		if err != nil {
			panic(err)
		}
		return modifiedJSON
	}

	return projectJSON
}

func AsServerResponsePlainArray(projectList []*projects.Project) []byte {
	var result []json.RawMessage
	for _, project := range projectList {
		result = append(result, AsServerResponse(project))
	}
	arrayJSON, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return arrayJSON
}

func AsServerResponseArray(projectList []*projects.Project) []byte {
	projectsJSON, err := json.Marshal(resources.Resources[*projects.Project]{
		Items: projectList,
	})
	if err != nil {
		panic(err)
	}

	var wrapper map[string]interface{}
	if err := json.Unmarshal(projectsJSON, &wrapper); err != nil {
		panic(err)
	}

	if items, ok := wrapper["Items"].([]interface{}); ok {
		for i, item := range items {
			if projectMap, ok := item.(map[string]interface{}); ok {
				if persistenceSettings, ok := projectMap["PersistenceSettings"].(map[string]interface{}); ok {
					if persistenceSettings["Type"] == "VersionControlled" {
						if i < len(projectList) {
							if gitSettings, ok := projectList[i].PersistenceSettings.(projects.GitPersistenceSettings); ok {
								persistenceSettings["ConversionState"] = map[string]interface{}{
									"VariablesAreInGit": gitSettings.VariablesAreInGit(),
									"RunbooksAreInGit":  gitSettings.RunbooksAreInGit(),
								}
							}
						}
					}
				}
			}
		}
	}

	modifiedJSON, err := json.Marshal(wrapper)
	if err != nil {
		panic(err)
	}
	return modifiedJSON
}

func NewChannel(spaceID string, channelID string, channelName string, projectID string) *channels.Channel {
	result := channels.NewChannel(channelName, projectID)
	result.ID = channelID
	result.SpaceID = spaceID
	return result
}

func NewEphemeralChannel(spaceID string, channelID string, channelName string, projectID string, ephemeralEnvironmentNameTemplate string, autoDeploy bool) *channels.Channel {
	result := channels.NewChannel(channelName, projectID)
	result.Type = channels.ChannelTypeEphemeral
	result.ID = channelID
	result.SpaceID = spaceID
	result.EphemeralEnvironmentNameTemplate = ephemeralEnvironmentNameTemplate
	result.AutomaticEphemeralEnvironmentDeployments = autoDeploy
	return result
}

func NewRelease(spaceID string, releaseID string, releaseVersion string, projectID string, channelID string) *releases.Release {
	result := releases.NewRelease(channelID, projectID, releaseVersion)
	result.ID = releaseID
	result.SpaceID = spaceID
	result.Links = map[string]string{
		constants.LinkProgression: fmt.Sprintf("/api/%s/releases/%s/progression", spaceID, releaseID),
	}
	return result
}

func NewEnvironment(spaceID string, envID string, name string) *environments.Environment {
	result := environments.NewEnvironment(name)
	result.ID = envID
	result.SpaceID = spaceID
	return result
}

func NewEphemeralEnvironment(spaceID string, envID string, name string, parentEnvironmentID string) *ephemeralenvironments.EphemeralEnvironment {
	result := ephemeralenvironments.NewEphemeralEnvironment(name, parentEnvironmentID, spaceID)
	result.ID = envID

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

func NewTenant(spaceID string, tenantID string, name string, tenantTags ...string) *tenants.Tenant {
	result := tenants.NewTenant(name)
	result.ID = tenantID
	result.SpaceID = spaceID
	result.TenantTags = tenantTags
	// doesn't have any ProjectEnvironments, will need to add them externally
	return result
}

func NewRunbook(spaceID string, projectID string, runbookID string, name string) *runbooks.Runbook {
	result := runbooks.NewRunbook(name, projectID)
	result.ID = runbookID
	result.SpaceID = spaceID
	return result
}

func NewRunbookSnapshot(projectID string, runbookID string, snapshotID string, name string) *runbooks.RunbookSnapshot {
	result := runbooks.NewRunbookSnapshot(name, projectID, runbookID)
	result.ID = snapshotID
	// runbook snapshots don't have their own explicit spaceID, they are a child of the parent runbook
	return result
}

func NewRunbookProcessForRunbook(spaceID string, projectID string, runbookID string) *runbooks.RunbookProcess {
	result := runbooks.NewRunbookProcess()
	result.SpaceID = spaceID
	result.ProjectID = projectID
	result.ID = "RunbookProcess-" + runbookID
	return result
}

func EmptyDeploymentPreviews() []*deployments.DeploymentPreview {
	// Define the Form instance
	formValues := map[string]string{}
	formElements := []*deployments.Element{}
	form := deployments.NewFormWithValuesAndElements(formValues, formElements)

	deploymentPreview := &deployments.DeploymentPreview{
		Form:                          form,
		StepsToExecute:                []*deployments.DeploymentTemplateStep{},
		UseGuidedFailureModeByDefault: false,
	}

	// Serialize the DeploymentPreview instances to JSON
	deploymentPreviews := []*deployments.DeploymentPreview{deploymentPreview}

	return deploymentPreviews
}

func NewDeploymentPreviews() []*deployments.DeploymentPreview {
	newDisplaySettings := &resources.DisplaySettings{}
	controlPromptNotRequired := deployments.NewControl("VariableValue", "Prompt not required", "Prompt not required", "", false, newDisplaySettings)
	controlScopedSensitive := deployments.NewControl("VariableValue", "Scoped Sensitive", "Scoped Sensitive", "", true, resources.NewDisplaySettings(resources.ControlTypeSensitive, nil))

	elementPromptNotRequired := deployments.NewElement("5103b61d-9142-c146-d2c9-11b0e63aa438", controlPromptNotRequired, false)
	elementScopedSensitive := deployments.NewElement("41824a1b-64ad-430f-862b-39d8cfeeb13a", controlScopedSensitive, true)

	// Define the Form instance
	formValues := map[string]string{
		"5103b61d-9142-c146-d2c9-11b0e63aa438": "Prompt value not required",
		"41824a1b-64ad-430f-862b-39d8cfeeb13a": "Prompt secret value",
	}
	formElements := []*deployments.Element{elementPromptNotRequired, elementScopedSensitive}
	form := deployments.NewFormWithValuesAndElements(formValues, formElements)

	deploymentPreview1 := &deployments.DeploymentPreview{
		Form:                          form,
		StepsToExecute:                []*deployments.DeploymentTemplateStep{},
		UseGuidedFailureModeByDefault: false,
	}

	deploymentPreview2 := &deployments.DeploymentPreview{
		Form:                          form,
		StepsToExecute:                []*deployments.DeploymentTemplateStep{},
		UseGuidedFailureModeByDefault: false,
	}

	// Serialize the DeploymentPreview instances to JSON
	deploymentPreviews := []*deployments.DeploymentPreview{deploymentPreview1, deploymentPreview2}

	return deploymentPreviews
}

func NewDeploymentPreviewsWithApproval() []*deployments.DeploymentPreview {
	newDisplaySettings := &resources.DisplaySettings{}

	approvalPromptRequired := deployments.NewControl("VariableValue", "Approver", "Who approved this deployment?", "Who approved this deployment?", true, newDisplaySettings)
	approvalPromptRequiredElement := deployments.NewElement("1953afe6-f094-1287-2d8a-04846dc0f9b1", approvalPromptRequired, true)

	// Define the Form instance
	formValues := map[string]string{
		"1953afe6-f094-1287-2d8a-04846dc0f9b1": "",
	}
	formElements := []*deployments.Element{approvalPromptRequiredElement}
	form := deployments.NewFormWithValuesAndElements(formValues, formElements)

	deploymentPreview1 := &deployments.DeploymentPreview{
		Form:                          form,
		StepsToExecute:                []*deployments.DeploymentTemplateStep{},
		UseGuidedFailureModeByDefault: false,
	}

	// Serialize the DeploymentPreview instances to JSON
	deploymentPreviews := []*deployments.DeploymentPreview{deploymentPreview1}

	return deploymentPreviews
}
