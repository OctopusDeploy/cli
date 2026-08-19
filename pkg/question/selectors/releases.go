package selectors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
)

// latestReleaseAlias is the value the old `octo` CLI accepted to mean "the newest release".
// This CLI has no equivalent, so it is called out explicitly when the lookup fails.
const latestReleaseAlias = "latest"

// FindRelease looks up a release by version within a project. A version that doesn't exist is
// reported here, because the executions API answers one with a null reference error instead.
func FindRelease(octopus *octopusApiClient.Client, spaceID string, project *projects.Project, releaseVersion string) (*releases.Release, error) {
	release, err := releases.GetReleaseInProject(octopus, spaceID, project.GetID(), releaseVersion)
	if err != nil {
		var apiError *core.APIError
		if errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound {
			return nil, releaseNotFoundError(project, releaseVersion)
		}
		return nil, err
	}
	// a 404 with an empty body doesn't reach the error path above; it decodes as an empty release
	if release == nil || release.GetID() == "" {
		return nil, releaseNotFoundError(project, releaseVersion)
	}

	return release, nil
}

func releaseNotFoundError(project *projects.Project, releaseVersion string) error {
	if strings.EqualFold(releaseVersion, latestReleaseAlias) {
		return fmt.Errorf("cannot find a release with version '%s' in project '%s'; '%s' is not a supported alias, specify an exact version. Run '%s release list --project \"%s\"' to see the available versions",
			releaseVersion, project.GetName(), releaseVersion, constants.ExecutableName, project.GetName())
	}
	return fmt.Errorf("cannot find a release with version '%s' in project '%s'", releaseVersion, project.GetName())
}
