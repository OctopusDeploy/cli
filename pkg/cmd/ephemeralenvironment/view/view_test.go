package view_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
)

var serverUrl, _ = url.Parse("http://server")

const (
	placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	testSpaceID       = "Spaces-1"
	envName           = "PR-1234"
	envID             = "Environments-1"
	projectName       = "Test Project"
	projectID         = "Projects-1"
)

var rootResource = testutil.NewRootResource()

func setupMockAPI(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/"+testSpaceID).RespondWith(rootResource)
}

func mockGetProjectByName(t *testing.T, api *testutil.MockHttpServer) {
	project := projects.NewProject(projectName, "Lifecycles-1", "ProjectGroups-1")
	project.ID = projectID

	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects?partialName=%s", testSpaceID, url.PathEscape(projectName))).
		RespondWith(&resources.Resources[*projects.Project]{
			Items:        []*projects.Project{project},
			PagedResults: resources.PagedResults{TotalResults: 1},
		})
}

func mockGetEnvironmentByName(t *testing.T, api *testutil.MockHttpServer, envs []*ephemeralenvironments.EphemeralEnvironment) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?skip=0&take=2147483647&partialName=%s&type=Ephemeral", testSpaceID, envName)).
		RespondWith(&resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items:        envs,
			PagedResults: resources.PagedResults{TotalResults: len(envs)},
		})
}

func testEnvironment() *ephemeralenvironments.EphemeralEnvironment {
	env := ephemeralenvironments.NewEphemeralEnvironment(envName, "Environments-0", testSpaceID)
	env.ID = envID
	return env
}

func createOptions(t *testing.T, api *testutil.MockHttpServer, name string, project string, outputFormat string) (*view.ViewOptions, *bytes.Buffer) {
	octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
	assert.NoError(t, err)

	space := spaces.NewSpace("Test Space")
	space.ID = testSpaceID

	buf := new(bytes.Buffer)
	command := &cobra.Command{}
	command.Flags().String(constants.FlagOutputFormat, outputFormat, "")
	command.SetOut(buf)

	flags := view.NewViewFlags()
	flags.Name.Value = name
	flags.Project.Value = project

	deps := &cmd.Dependencies{
		Client:   octopus,
		Space:    space,
		NoPrompt: true,
		Out:      buf,
	}

	return view.NewViewOptions(flags, deps, command), buf
}

func TestView_Table(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, buf := createOptions(t, api, envName, projectName, constants.OutputFormatTable)

		err := view.ViewRun(opts)
		assert.NoError(t, err)

		assert.Contains(t, buf.String(), envName)
		assert.Contains(t, buf.String(), envID)
		assert.Contains(t, buf.String(), projectName)
		assert.Contains(t, buf.String(), "ProvisioningFailed")

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects/%s/environments/ephemeral/%s/status", testSpaceID, projectID, envID)).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "ProvisioningFailed"})

	assert.NoError(t, <-errReceiver)
}

func TestView_Json(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, buf := createOptions(t, api, envName, projectName, constants.OutputFormatJson)

		err := view.ViewRun(opts)
		assert.NoError(t, err)

		var result view.ViewResult
		assert.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, view.ViewResult{
			Id:          envID,
			Name:        envName,
			ProjectId:   projectID,
			ProjectName: projectName,
			Status:      "Provisioned",
		}, result)

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects/%s/environments/ephemeral/%s/status", testSpaceID, projectID, envID)).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "Provisioned"})

	assert.NoError(t, <-errReceiver)
}

func TestView_EnvironmentNotFound(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, _ := createOptions(t, api, envName, projectName, constants.OutputFormatTable)

		err := view.ViewRun(opts)
		assert.EqualError(t, err, fmt.Sprintf("no ephemeral environment found with the name '%s'", envName))

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{})

	assert.NoError(t, <-errReceiver)
}

func TestView_StatusError(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, _ := createOptions(t, api, envName, projectName, constants.OutputFormatTable)

		err := view.ViewRun(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get the status of ephemeral environment")

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects/%s/environments/ephemeral/%s/status", testSpaceID, projectID, envID)).
		RespondWithStatus(404, "404 Not Found", struct{ ErrorMessage string }{"not found"})

	assert.NoError(t, <-errReceiver)
}

func TestView_RequiresName(t *testing.T) {
	api := testutil.NewMockHttpServer()
	defer api.Close()

	opts, _ := createOptions(t, api, "", projectName, constants.OutputFormatTable)

	assert.EqualError(t, view.ViewRun(opts), "environment name is required")
}

func TestView_RequiresProject(t *testing.T) {
	api := testutil.NewMockHttpServer()
	defer api.Close()

	opts, _ := createOptions(t, api, envName, "", constants.OutputFormatTable)

	assert.EqualError(t, view.ViewRun(opts), "project name is required")
}
