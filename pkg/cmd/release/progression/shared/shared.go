package shared

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
)

func GetReleaseID(octopus *client.Client, spaceID string, projectIdentifier string, version string) (string, error) {
	selectedProject, err := selectors.FindProject(octopus, projectIdentifier)
	if err != nil {
		return "", err
	}

	selectedRelease, err := FindRelease(octopus, selectedProject, version)
	if err != nil {
		return "", err
	}
	return selectedRelease.GetID(), nil
}

func SelectRelease(octopus *client.Client, project *projects.Project, ask question.Asker, action string) (*releases.Release, error) {
	existingReleases, err := octopus.Projects.GetReleases(project)
	if err != nil {
		return nil, err
	}

	selectedRelease, err := question.SelectMap(ask, fmt.Sprintf("Select Release to %s Progression for", action), existingReleases, func(r *releases.Release) string {
		return r.Version
	})
	if err != nil {
		return nil, err
	}

	return selectedRelease, nil
}

func FindRelease(octopus *client.Client, project *projects.Project, version string) (*releases.Release, error) {
	existingRelease, err := releases.GetReleaseInProject(octopus, octopus.GetSpaceID(), project.GetID(), version)
	if err != nil {
		return nil, err
	}

	if existingRelease == nil {
		return nil, fmt.Errorf("unable to locate a release with version/release number '%s'", version)
	}

	return existingRelease, nil
}
