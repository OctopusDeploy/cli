package selectors

import (
	"errors"

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

// ResolveProject finds the project a command should operate on: looking it up when
// the caller named one, prompting for it in interactive mode when they didn't, and
// failing in automation mode where there is nobody to ask.
//
// Callers that echo the resolved project can do so on projectIdentifier != "",
// which is exactly the case where no prompt was shown.
func ResolveProject(octopus *octopusApiClient.Client, ask question.Asker, promptEnabled bool, questionText string, projectIdentifier string) (*projects.Project, error) {
	if projectIdentifier == "" {
		if !promptEnabled {
			return nil, errors.New("project must be specified")
		}
		return Project(questionText, octopus, ask)
	}
	return FindProject(octopus, projectIdentifier)
}
