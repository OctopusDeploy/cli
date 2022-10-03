package selectors

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"io"
	"strings"
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

func FindProject(octopus *octopusApiClient.Client, projectName string) (*projects.Project, error) {
	// projectsQuery has "Name" but it's just an alias in the server for PartialName; we need to filter client side
	projectsPage, err := octopus.Projects.Get(projects.ProjectsQuery{PartialName: projectName})
	if err != nil {
		return nil, err
	}
	for projectsPage != nil && len(projectsPage.Items) > 0 {
		for _, c := range projectsPage.Items { // server doesn't support channel search by exact name so we must emulate it
			if strings.EqualFold(c.Name, projectName) {
				return c, nil
			}
		}
		projectsPage, err = projectsPage.GetNextPage(octopus.Projects.GetClient())
		if err != nil {
			return nil, err
		} // if there are no more pages, then GetNextPage will return nil, which breaks us out of the loop
	}

	return nil, fmt.Errorf("no project found with name of %s", projectName)
}

// SelectOrFindProject is a very specific high level helper that drops into a standard AskQuestions workflow.
// If projectNameOrID is blank, it will present a single-select list and ask the user to pick one, then return it.
// If non-blank, it will search for a project on the server with that name, print the result, then return it.
//
// Think carefully before using this, if you are not inside a Command AskQuestions function, you probably want one of the lower level selectors instead
func SelectOrFindProject(projectNameOrID string, questionText string, octopus *octopusApiClient.Client, asker question.Asker, stdout io.Writer, outputFormat string) (*projects.Project, error) {
	if projectNameOrID == "" {
		return Project(questionText, octopus, asker)
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err := FindProject(octopus, projectNameOrID)
		if err != nil {
			return nil, err
		}

		if !constants.IsProgrammaticOutputFormat(outputFormat) {
			_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
		}
		return selectedProject, nil
	}
}
