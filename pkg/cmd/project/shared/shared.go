package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

type CreateProjectGroupCallback func() (string, cmd.Dependable, error)
type GetAllGroupsCallback func() ([]*projectgroups.ProjectGroup, error)

func GetAllGroups(client client.Client) ([]*projectgroups.ProjectGroup, error) {
	res, err := client.ProjectGroups.GetAll()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func CreateProjectGroup(dependencies *cmd.Dependencies) (string, cmd.Dependable, error) {
	optValues := projectGroupCreate.NewCreateFlags()
	projectGroupOpts := cmd.NewDependenciesFromExisting(dependencies, "octopus project-group create")

	projectGroupCreateOpts := projectGroupCreate.NewCreateOptions(optValues, projectGroupOpts)
	projectGroupCreate.PromptMissing(projectGroupCreateOpts)
	returnValue := projectGroupCreateOpts.Name.Value
	return returnValue, projectGroupCreateOpts, nil
}

func AskProjectGroups(ask question.Asker, value string, getAllGroupsCallback GetAllGroupsCallback, createProjectGroupCallback CreateProjectGroupCallback) (string, cmd.Dependable, error) {
	if value != "" {
		return value, nil, nil
	}
	g, shouldCreateNew, err := selectors.SelectOrNew(ask, "You have not specified a Project group for this project. Please select one:", getAllGroupsCallback, func(pg *projectgroups.ProjectGroup) string {
		return pg.Name
	})
	if err != nil {
		return "", nil, err
	}
	if shouldCreateNew {
		return createProjectGroupCallback()
	}
	return g.Name, nil, nil
}

// GetLifecycleMap resolves lifecycle IDs to names for display. Best-effort: a
// failed lookup yields an empty map and callers fall back to the ID.
func GetLifecycleMap(octopus *client.Client) map[string]string {
	lifecycleMap := make(map[string]string)
	allLifecycles, err := octopus.Lifecycles.GetAll()
	if err != nil {
		return lifecycleMap
	}
	for _, l := range allLifecycles {
		lifecycleMap[l.GetID()] = l.Name
	}
	return lifecycleMap
}

// GetProjectGroupMap resolves project group IDs to names for display. Best-effort,
// as GetLifecycleMap is.
func GetProjectGroupMap(octopus *client.Client) map[string]string {
	projectGroupMap := make(map[string]string)
	allProjectGroups, err := octopus.ProjectGroups.GetAll()
	if err != nil {
		return projectGroupMap
	}
	for _, pg := range allProjectGroups {
		projectGroupMap[pg.GetID()] = pg.Name
	}
	return projectGroupMap
}

// GetLifecycleName resolves a single lifecycle ID, which is cheaper than a whole
// map when only one project is being displayed. Empty when it can't be resolved.
func GetLifecycleName(octopus *client.Client, lifecycleID string) string {
	if lifecycleID == "" {
		return ""
	}
	lifecycle, err := octopus.Lifecycles.GetByID(lifecycleID)
	if err != nil {
		return ""
	}
	return lifecycle.Name
}

// GetProjectGroupName resolves a single project group ID, as GetLifecycleName does.
func GetProjectGroupName(octopus *client.Client, projectGroupID string) string {
	if projectGroupID == "" {
		return ""
	}
	projectGroup, err := octopus.ProjectGroups.GetByID(projectGroupID)
	if err != nil {
		return ""
	}
	return projectGroup.Name
}

// DisplayName prefers the resolved name, falling back to the ID so there is always
// something to show.
func DisplayName(id string, name string) string {
	if name == "" {
		return id
	}
	return name
}

// TenantedDeploymentMode reports the project's mode, defaulting to Untenanted as
// the server does when the project doesn't carry one.
func TenantedDeploymentMode(project *projects.Project) string {
	if project.TenantedDeploymentMode == "" {
		return string(core.TenantedDeploymentModeUntenanted)
	}
	return string(project.TenantedDeploymentMode)
}
