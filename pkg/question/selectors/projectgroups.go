package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
)

func ProjectGroup(questionText string, octopus *client.Client, ask question.Asker) (*projectgroups.ProjectGroup, error) {
	existingGroups, err := octopus.ProjectGroups.GetAll()
	if err != nil {
		return nil, err
	}

	return Select(ask, questionText, existingGroups, func(lc *projectgroups.ProjectGroup) string {
		return lc.Name
	})
}
