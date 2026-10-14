package list_test

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
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
)

var serverUrl, _ = url.Parse("http://server")

const (
	placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	testSpaceID       = "Spaces-1"
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

func testEnvironment(id string, name string) *ephemeralenvironments.EphemeralEnvironment {
	env := ephemeralenvironments.NewEphemeralEnvironment(name, "Environments-0", testSpaceID)
	env.ID = id
	return env
}

func mockGetAllEnvironments(t *testing.T, api *testutil.MockHttpServer, envs []*ephemeralenvironments.EphemeralEnvironment) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?skip=0&take=2147483647&type=Ephemeral", testSpaceID)).
		RespondWith(&resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items:        envs,
			PagedResults: resources.PagedResults{TotalResults: len(envs)},
		})
}

func statusPath(environmentID string) string {
	return fmt.Sprintf("/api/%s/projects/%s/environments/ephemeral/%s/status", testSpaceID, projectID, environmentID)
}

func createOptions(t *testing.T, api *testutil.MockHttpServer, project string, outputFormat string) (*list.ListOptions, *bytes.Buffer, *bytes.Buffer) {
	octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
	assert.NoError(t, err)

	space := spaces.NewSpace("Test Space")
	space.ID = testSpaceID

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := &cobra.Command{}
	command.Flags().String(constants.FlagOutputFormat, outputFormat, "")
	command.SetOut(stdout)
	command.SetErr(stderr)

	flags := list.NewListFlags()
	flags.Project.Value = project

	deps := &cmd.Dependencies{
		Client:   octopus,
		Space:    space,
		NoPrompt: true,
		Out:      stdout,
	}

	return list.NewListOptions(flags, deps, command), stdout, stderr
}

func TestList_WithoutProjectHasNoStatus(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, stdout, _ := createOptions(t, api, "", constants.OutputFormatTable)

		assert.NoError(t, list.ListRun(opts))

		assert.Contains(t, stdout.String(), "PR-1")
		assert.Contains(t, stdout.String(), "Environments-1")
		assert.Contains(t, stdout.String(), "PR-2")
		assert.NotContains(t, stdout.String(), "STATUS")

		return nil
	})

	setupMockAPI(t, api)
	mockGetAllEnvironments(t, api, []*ephemeralenvironments.EphemeralEnvironment{
		testEnvironment("Environments-1", "PR-1"),
		testEnvironment("Environments-2", "PR-2"),
	})

	assert.NoError(t, <-errReceiver)
}

func TestList_WithProjectShowsStatus(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, stdout, _ := createOptions(t, api, projectName, constants.OutputFormatTable)

		assert.NoError(t, list.ListRun(opts))

		assert.Contains(t, stdout.String(), "STATUS")
		assert.Contains(t, stdout.String(), "Provisioned")
		assert.Contains(t, stdout.String(), "ProvisioningFailed")

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetAllEnvironments(t, api, []*ephemeralenvironments.EphemeralEnvironment{
		testEnvironment("Environments-1", "PR-1"),
		testEnvironment("Environments-2", "PR-2"),
	})
	api.ExpectRequest(t, "GET", statusPath("Environments-1")).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "Provisioned"})
	api.ExpectRequest(t, "GET", statusPath("Environments-2")).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "ProvisioningFailed"})

	assert.NoError(t, <-errReceiver)
}

func TestList_Json(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, stdout, _ := createOptions(t, api, projectName, constants.OutputFormatJson)

		assert.NoError(t, list.ListRun(opts))

		var result []list.EphemeralEnvironmentViewModel
		assert.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
		assert.Equal(t, []list.EphemeralEnvironmentViewModel{
			{Id: "Environments-1", Name: "PR-1", Status: "Provisioned"},
		}, result)

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetAllEnvironments(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment("Environments-1", "PR-1")})
	api.ExpectRequest(t, "GET", statusPath("Environments-1")).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "Provisioned"})

	assert.NoError(t, <-errReceiver)
}

func TestList_JsonWithoutProjectOmitsStatus(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, stdout, _ := createOptions(t, api, "", constants.OutputFormatJson)

		assert.NoError(t, list.ListRun(opts))
		assert.NotContains(t, stdout.String(), "Status")

		return nil
	})

	setupMockAPI(t, api)
	mockGetAllEnvironments(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment("Environments-1", "PR-1")})

	assert.NoError(t, <-errReceiver)
}

func TestList_StatusFailureDoesNotStopListing(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, stdout, stderr := createOptions(t, api, projectName, constants.OutputFormatTable)

		assert.NoError(t, list.ListRun(opts))

		assert.Contains(t, stdout.String(), "PR-1")
		assert.Contains(t, stdout.String(), list.StatusUnknown)
		assert.Contains(t, stdout.String(), "PR-2")
		assert.Contains(t, stdout.String(), "Provisioned")
		assert.Contains(t, stderr.String(), "Could not get the status of ephemeral environment 'PR-1'")

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api)
	mockGetAllEnvironments(t, api, []*ephemeralenvironments.EphemeralEnvironment{
		testEnvironment("Environments-1", "PR-1"),
		testEnvironment("Environments-2", "PR-2"),
	})
	api.ExpectRequest(t, "GET", statusPath("Environments-1")).
		RespondWithStatus(404, "404 Not Found", struct{ ErrorMessage string }{"not found"})
	api.ExpectRequest(t, "GET", statusPath("Environments-2")).
		RespondWith(&ephemeralenvironments.EphemeralEnvironmentStatus{Status: "Provisioned"})

	assert.NoError(t, <-errReceiver)
}

func TestList_ProjectNotFound(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, _, _ := createOptions(t, api, projectName, constants.OutputFormatTable)

		err := list.ListRun(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("failed to find project '%s'", projectName))

		return nil
	})

	setupMockAPI(t, api)
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects?partialName=%s", testSpaceID, url.PathEscape(projectName))).
		RespondWith(&resources.Resources[*projects.Project]{
			Items:        []*projects.Project{},
			PagedResults: resources.PagedResults{TotalResults: 0},
		})

	assert.NoError(t, <-errReceiver)
}
