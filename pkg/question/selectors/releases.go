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

// ReleaseNotFoundError reports a release version that the server didn't return a release for.
//
// Confirmed distinguishes the two ways that answer arrives. A 404 carrying an APIError body is a
// definite "no such release". An empty response body is not: the SDK's DoRawJsonRequest short-circuits
// on `resp.ContentLength == 0` and returns (resp, nil) for *any* status code, so a 403 with no body, or
// a 502 from a proxy, decodes into a zero-valued Release with a nil error and is indistinguishable from
// a 404 by the time it reaches us. The status code isn't recoverable at this layer, so the message
// hedges rather than asserting the release is missing.
type ReleaseNotFoundError struct {
	ProjectName    string
	ReleaseVersion string
	Confirmed      bool
}

func (e *ReleaseNotFoundError) Error() string {
	var message string
	if e.Confirmed {
		message = fmt.Sprintf("cannot find a release with version '%s' in project '%s'", e.ReleaseVersion, e.ProjectName)
	} else {
		message = fmt.Sprintf("could not resolve a release with version '%s' in project '%s'; the server returned an empty response, which usually means there is no such release, but can also mean the lookup itself failed", e.ReleaseVersion, e.ProjectName)
	}

	if strings.EqualFold(e.ReleaseVersion, latestReleaseAlias) {
		message += fmt.Sprintf(". '%s' is not a supported alias, specify an exact version. Run '%s release list --project \"%s\"' to see the available versions",
			e.ReleaseVersion, constants.ExecutableName, e.ProjectName)
	}

	return message
}

// FindRelease looks up a release by version within a project. A version that doesn't exist is
// reported here, because the executions API answers one with a null reference error instead.
// Anything else (a permissions failure, a transport error) is returned untouched, so callers that
// would rather let the server be the authority can tell the two apart with errors.As.
func FindRelease(octopus *octopusApiClient.Client, spaceID string, project *projects.Project, releaseVersion string) (*releases.Release, error) {
	release, err := releases.GetReleaseInProject(octopus, spaceID, project.GetID(), releaseVersion)
	if err != nil {
		var apiError *core.APIError
		if errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound {
			return nil, &ReleaseNotFoundError{ProjectName: project.GetName(), ReleaseVersion: releaseVersion, Confirmed: true}
		}
		return nil, err
	}
	// an empty response body doesn't reach the error path above; it decodes as an empty release.
	// See ReleaseNotFoundError for why this can't be reported as a definite "not found".
	if release == nil || release.GetID() == "" {
		return nil, &ReleaseNotFoundError{ProjectName: project.GetName(), ReleaseVersion: releaseVersion}
	}

	return release, nil
}
