package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
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
