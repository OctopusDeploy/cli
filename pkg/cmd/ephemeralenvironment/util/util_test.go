package util_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("http://server")

const (
	placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	testSpaceID       = "Spaces-1"
)

var rootResource = testutil.NewRootResource()

// Helper function to create test ephemeral environments
func createTestEnvironment(id, name string) *ephemeralenvironments.EphemeralEnvironment {
	env := ephemeralenvironments.NewEphemeralEnvironment(name, "Environments-1", testSpaceID)
	env.ID = id
	return env
}

// Helper function to set up API mock with initial requests
func setupMockAPI(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/"+testSpaceID).RespondWith(rootResource)
}

// Helper function to mock the GetByPartialName API response
func mockGetByPartialName(t *testing.T, api *testutil.MockHttpServer, searchName string, envs []*ephemeralenvironments.EphemeralEnvironment) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?skip=0&take=2147483647&partialName=%s&type=Ephemeral", testSpaceID, searchName)).
		RespondWith(&resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items:        envs,
			PagedResults: resources.PagedResults{TotalResults: len(envs)},
		})
}

// Integration tests using MockHttpServer
func TestGetByName_SingleMatch(t *testing.T) {
	const envName = "test-env"
	env1 := createTestEnvironment("Environments-1", envName)

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		if err != nil {
			return err
		}

		result, err := util.GetByName(octopus, envName, testSpaceID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		if result != nil {
			assert.Equal(t, env1.ID, result.ID)
			assert.Equal(t, env1.Name, result.Name)
		}
		return nil
	})

	setupMockAPI(t, api)
	mockGetByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env1})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestGetByName_MultipleMatchesWithExactMatch(t *testing.T) {
	const searchName = "test-env"
	env1 := createTestEnvironment("Environments-1", "test-env")
	env2 := createTestEnvironment("Environments-2", "test-env-2")
	env3 := createTestEnvironment("Environments-3", "test-environment")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		if err != nil {
			return err
		}

		result, err := util.GetByName(octopus, searchName, testSpaceID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		if result != nil {
			assert.Equal(t, env1.ID, result.ID)
			assert.Equal(t, env1.Name, result.Name)
		}
		return nil
	})

	setupMockAPI(t, api)
	mockGetByPartialName(t, api, searchName, []*ephemeralenvironments.EphemeralEnvironment{env1, env2, env3})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestGetByName_MultipleMatchesCaseInsensitive(t *testing.T) {
	const searchName = "test-env"
	// The actual environment name has different case
	env1 := createTestEnvironment("Environments-1", "Test-Env")
	env2 := createTestEnvironment("Environments-2", "test-env-2")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		if err != nil {
			return err
		}

		result, err := util.GetByName(octopus, searchName, testSpaceID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		if result != nil {
			assert.Equal(t, env1.ID, result.ID)
			assert.Equal(t, "Test-Env", result.Name) // Should return the actual name with its original casing
		}
		return nil
	})

	setupMockAPI(t, api)
	mockGetByPartialName(t, api, searchName, []*ephemeralenvironments.EphemeralEnvironment{env1, env2})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestGetByName_MultipleMatchesWithoutExactMatch(t *testing.T) {
	const searchName = "test"
	env1 := createTestEnvironment("Environments-1", "test-env")
	env2 := createTestEnvironment("Environments-2", "test-env-2")
	env3 := createTestEnvironment("Environments-3", "test-environment")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		if err != nil {
			return err
		}

		result, err := util.GetByName(octopus, searchName, testSpaceID)
		assert.Error(t, err)
		assert.Nil(t, result)
		if err != nil {
			expectedError := fmt.Sprintf("could not find an exact match of an ephemeral environment with the name '%s'. Please specify a more specific name", searchName)
			assert.Equal(t, expectedError, err.Error())
		}
		return nil
	})

	setupMockAPI(t, api)
	mockGetByPartialName(t, api, searchName, []*ephemeralenvironments.EphemeralEnvironment{env1, env2, env3})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestGetByName_NoResults(t *testing.T) {
	const searchName = "nonexistent"

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		if err != nil {
			return err
		}

		result, err := util.GetByName(octopus, searchName, testSpaceID)

		assert.Error(t, err)
		assert.Nil(t, result)
		if err != nil {
			expectedError := fmt.Sprintf("no ephemeral environment found with the name '%s'", searchName)
			assert.Equal(t, expectedError, err.Error())
		}
		return nil
	})

	// Mock the initial API requests
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/"+testSpaceID).RespondWith(rootResource)

	// Mock the GetByPartialName API call returning no results
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?skip=0&take=2147483647&partialName=%s&type=Ephemeral", testSpaceID, searchName)).
		RespondWith(&resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items:        []*ephemeralenvironments.EphemeralEnvironment{},
			PagedResults: resources.PagedResults{TotalResults: 0},
		})

	err := <-errReceiver
	assert.NoError(t, err)
}
