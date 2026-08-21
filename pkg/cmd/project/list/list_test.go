package list

import (
	"errors"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/stretchr/testify/assert"
)

func newProject(name string) *projects.Project {
	return projects.NewProject(name, "Lifecycles-1", "ProjectGroups-1")
}

func TestGetProjects_WithoutGroupListsEverything(t *testing.T) {
	getAllProjects := func() ([]*projects.Project, error) {
		return []*projects.Project{newProject("foo"), newProject("bar")}, nil
	}
	getProjectGroup := func(idOrName string) (*projectgroups.ProjectGroup, error) {
		t.Errorf("did not expect a project group lookup, got '%s'", idOrName)
		return nil, nil
	}
	getProjectsInGroup := func(projectGroup *projectgroups.ProjectGroup) ([]*projects.Project, error) {
		t.Error("did not expect the projects in a group to be requested")
		return nil, nil
	}

	result, err := getProjects("", getAllProjects, getProjectGroup, getProjectsInGroup)

	assert.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, projectNames(result))
}

func TestGetProjects_WithGroupListsOnlyThatGroup(t *testing.T) {
	getAllProjects := func() ([]*projects.Project, error) {
		t.Error("did not expect every project to be requested")
		return nil, nil
	}
	getProjectGroup := func(idOrName string) (*projectgroups.ProjectGroup, error) {
		assert.Equal(t, "Default Project Group", idOrName)
		return projectgroups.NewProjectGroup("Default Project Group"), nil
	}
	getProjectsInGroup := func(projectGroup *projectgroups.ProjectGroup) ([]*projects.Project, error) {
		assert.Equal(t, "Default Project Group", projectGroup.Name)
		return []*projects.Project{newProject("foo")}, nil
	}

	result, err := getProjects("Default Project Group", getAllProjects, getProjectGroup, getProjectsInGroup)

	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, projectNames(result))
}

func TestGetProjects_WithUnknownGroupReportsTheGroup(t *testing.T) {
	getProjectGroup := func(idOrName string) (*projectgroups.ProjectGroup, error) {
		return nil, services.ErrItemNotFound
	}

	result, err := getProjects("Nope", nil, getProjectGroup, nil)

	assert.Nil(t, result)
	assert.EqualError(t, err, "cannot find a project group with name or ID of 'Nope'")
}

func TestGetProjects_WithNilGroupReportsTheGroup(t *testing.T) {
	getProjectGroup := func(idOrName string) (*projectgroups.ProjectGroup, error) {
		return nil, nil
	}

	result, err := getProjects("Nope", nil, getProjectGroup, nil)

	assert.Nil(t, result)
	assert.EqualError(t, err, "cannot find a project group with name or ID of 'Nope'")
}

func TestGetProjects_SurfacesOtherLookupErrors(t *testing.T) {
	expected := errors.New("the remote server returned 401 unauthorized")
	getProjectGroup := func(idOrName string) (*projectgroups.ProjectGroup, error) {
		return nil, expected
	}

	result, err := getProjects("Default Project Group", nil, getProjectGroup, nil)

	assert.Nil(t, result)
	assert.Equal(t, expected, err)
}

func projectNames(items []*projects.Project) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}
