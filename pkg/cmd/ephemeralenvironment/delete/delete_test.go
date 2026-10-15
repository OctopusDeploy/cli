package delete_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	deleteCmd "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/delete"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/test/testutil"
)

var serverUrl, _ = url.Parse("http://server")

const (
	placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	testSpaceID       = "Spaces-1"
	envName           = "PR-1234"
	envID             = "Environments-1001"
)

var rootResource = testutil.NewRootResource()

func setupMockAPI(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/"+testSpaceID).RespondWith(rootResource)
}

func mockGetEnvironmentByName(t *testing.T, api *testutil.MockHttpServer, envs []*ephemeralenvironments.EphemeralEnvironment) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?name=&skip=0&take=2147483647&partialName=%s&type=Ephemeral", testSpaceID, envName)).
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

func createOptions(t *testing.T, api *testutil.MockHttpServer, name string, confirm bool, noPrompt bool, ask func(survey.Prompt, any, ...survey.AskOpt) error) (*deleteCmd.DeleteOptions, *bytes.Buffer) {
	octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
	assert.NoError(t, err)

	space := spaces.NewSpace("Test Space")
	space.ID = testSpaceID

	buf := new(bytes.Buffer)

	flags := deleteCmd.NewDeleteFlags()
	flags.Name.Value = name
	flags.Confirm.Value = confirm

	deps := &cmd.Dependencies{
		Client:   octopus,
		Space:    space,
		NoPrompt: noPrompt,
		Out:      buf,
		Ask:      ask,
	}

	return deleteCmd.NewDeleteOptions(flags, deps), buf
}

func TestDelete_WithConfirmFlag(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, buf := createOptions(t, api, envName, true, true, nil)

		assert.NoError(t, deleteCmd.DeleteRun(opts))
		assert.Contains(t, buf.String(), envName)
		assert.Contains(t, buf.String(), envID)

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})
	api.ExpectRequest(t, "DELETE", "/api/"+testSpaceID+"/environments/"+envID).RespondWith(nil)

	assert.NoError(t, <-errReceiver)
}

func TestDelete_NoPromptWithoutConfirmFlagDoesNotDelete(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		// with prompting disabled the asker reports that it cannot prompt
		ask := func(survey.Prompt, any, ...survey.AskOpt) error {
			return &cliErrors.PromptDisabledError{}
		}
		opts, _ := createOptions(t, api, envName, false, true, ask)

		err := deleteCmd.DeleteRun(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--confirm")

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})

	assert.NoError(t, <-errReceiver)
}

func TestDelete_PromptsForConfirmation(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()

		asked := 0
		ask := func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
			asked++
			*(response.(*string)) = envName
			return nil
		}
		opts, _ := createOptions(t, api, envName, false, false, ask)

		assert.NoError(t, deleteCmd.DeleteRun(opts))
		assert.Equal(t, 1, asked)

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})
	api.ExpectRequest(t, "DELETE", "/api/"+testSpaceID+"/environments/"+envID).RespondWith(nil)

	assert.NoError(t, <-errReceiver)
}

func TestDelete_WrongConfirmationNameDoesNotDelete(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()

		ask := func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
			*(response.(*string)) = "something-else"
			return nil
		}
		opts, _ := createOptions(t, api, envName, false, false, ask)

		assert.Error(t, deleteCmd.DeleteRun(opts))

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{testEnvironment()})

	assert.NoError(t, <-errReceiver)
}

func TestDelete_EnvironmentNotFound(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, _ := createOptions(t, api, envName, true, true, nil)

		assert.EqualError(t, deleteCmd.DeleteRun(opts), fmt.Sprintf("no ephemeral environment found with the name '%s'", envName))

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvironmentByName(t, api, []*ephemeralenvironments.EphemeralEnvironment{})

	assert.NoError(t, <-errReceiver)
}

func TestDelete_RequiresName(t *testing.T) {
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		opts, _ := createOptions(t, api, "", true, true, nil)
		assert.EqualError(t, deleteCmd.DeleteRun(opts), "environment name is required")

		return nil
	})

	setupMockAPI(t, api)

	assert.NoError(t, <-errReceiver)
}
