package util

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"strings"
)

func SelectProject(questionText string, octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner) (*projects.Project, error) {
	spinner.Start()
	existingProjects, err := octopus.Projects.GetAll()
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string {
		return p.Name
	})
}

func FindProject(octopus *octopusApiClient.Client, spinner factory.Spinner, projectName string) (*projects.Project, error) {
	// projectsQuery has "Name" but it's just an alias in the server for PartialName; we need to filter client side
	spinner.Start()
	projectsPage, err := octopus.Projects.Get(projects.ProjectsQuery{PartialName: projectName})
	if err != nil {
		spinner.Stop()
		return nil, err
	}
	for projectsPage != nil && len(projectsPage.Items) > 0 {
		for _, c := range projectsPage.Items { // server doesn't support channel search by exact name so we must emulate it
			if strings.EqualFold(c.Name, projectName) {
				spinner.Stop()
				return c, nil
			}
		}
		projectsPage, err = projectsPage.GetNextPage(octopus.Projects.GetClient())
		if err != nil {
			spinner.Stop()
			return nil, err
		} // if there are no more pages, then GetNextPage will return nil, which breaks us out of the loop
	}

	spinner.Stop()
	return nil, fmt.Errorf("no project found with name of %s", projectName)
}
