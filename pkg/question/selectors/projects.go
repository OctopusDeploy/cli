package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

func Project(questionText string, octopus *octopusApiClient.Client, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := octopus.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string {
		return p.Name
	})
}

func FindProject(octopus *octopusApiClient.Client, projectIdentifier string) (*projects.Project, error) {
	project, err := octopus.Projects.GetByIdentifier(projectIdentifier)
	if err != nil {
		return nil, err
	}

	return project, nil
}
