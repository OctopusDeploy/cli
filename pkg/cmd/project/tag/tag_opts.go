package tag

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

type GetAllTagSetsCallback func() ([]*tagsets.TagSet, error)
type GetProjectCallback func(projectIdentifier string) (*projects.Project, error)
type GetProjectsCallback func() ([]*projects.Project, error)

type TagOptions struct {
	*TagFlags
	*cmd.Dependencies
	GetAllTagsCallback   GetAllTagSetsCallback
	GetProjectCallback   GetProjectCallback
	GetProjectsCallback  GetProjectsCallback
	project              *projects.Project
}

func NewTagOptions(tagFlags *TagFlags, dependencies *cmd.Dependencies) *TagOptions {
	return &TagOptions{
		TagFlags:            tagFlags,
		Dependencies:        dependencies,
		GetAllTagsCallback:  getAllTagSetsCallback(dependencies.Client),
		GetProjectCallback:  getProjectCallback(dependencies.Client),
		GetProjectsCallback: getProjectsCallback(dependencies.Client),
		project:             nil,
	}
}

func (to *TagOptions) Commit() error {
	if to.project == nil {
		project, err := to.GetProjectCallback(to.Project.Value)
		if err != nil {
			return err
		}
		to.project = project
	}

	to.project.ProjectTags = to.Tag.Value

	updatedProject, err := to.Client.Projects.Update(to.project)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(to.Out, "\nSuccessfully updated project %s (%s).\n", updatedProject.Name, updatedProject.ID)
	if err != nil {
		return err
	}
	return nil
}

func (to *TagOptions) GenerateAutomationCmd() {
	if !to.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(to.CmdPath, to.Project, to.Tag)
		fmt.Fprintf(to.Out, "%s\n", autoCmd)
	}
}

func getProjectCallback(client *client.Client) GetProjectCallback {
	return func(projectIdentifier string) (*projects.Project, error) {
		// Try to get by ID first
		project, _ := projects.GetByID(client, client.GetSpaceID(), projectIdentifier)
		if project != nil {
			return project, nil
		}

		// Fall back to lookup by name
		allProjects, err := projects.Get(client, client.GetSpaceID(), projects.ProjectsQuery{
			PartialName: projectIdentifier,
		})
		if err != nil {
			return nil, err
		}

		for _, proj := range allProjects.Items {
			if strings.EqualFold(proj.Name, projectIdentifier) {
				return proj, nil
			}
		}

		return nil, fmt.Errorf("project '%s' not found", projectIdentifier)
	}
}

func getProjectsCallback(client *client.Client) GetProjectsCallback {
	return func() ([]*projects.Project, error) {
		allProjects, err := projects.GetAll(client, client.GetSpaceID())
		if err != nil {
			return nil, err
		}
		return allProjects, nil
	}
}

func getAllTagSetsCallback(client *client.Client) GetAllTagSetsCallback {
	return func() ([]*tagsets.TagSet, error) {
		query := tagsets.TagSetsQuery{
			Scopes: []string{string(tagsets.TagSetScopeProject)},
		}
		result, err := tagsets.Get(client, client.GetSpaceID(), query)
		if err != nil {
			return nil, err
		}
		return result.Items, nil
	}
}
