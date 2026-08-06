package deprovision_environment_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	deprovisionEnvironment "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-environment"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
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

// Helper function to create a deprovision response
func createDeprovisionResponse(runbookRunID, taskID string) *ephemeralenvironments.DeprovisionEphemeralEnvironmentResponse {
	return &ephemeralenvironments.DeprovisionEphemeralEnvironmentResponse{
		DeprovisioningRuns: []ephemeralenvironments.DeprovisioningRunbookRun{
			{
				RunbookRunID: runbookRunID,
				TaskId:       taskID,
			},
		},
	}
}

// Helper function to set up API mock with initial requests
func setupMockAPI(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/"+testSpaceID).RespondWith(rootResource)
}

// Helper function to mock the GetByPartialName API response
func mockGetEnvByPartialName(t *testing.T, api *testutil.MockHttpServer, searchName string, envs []*ephemeralenvironments.EphemeralEnvironment) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/environments/v2?skip=0&take=2147483647&partialName=%s&type=Ephemeral", testSpaceID, searchName)).
		RespondWith(&resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items:        envs,
			PagedResults: resources.PagedResults{TotalResults: len(envs)},
		})
}

// Helper function to mock the Deprovision API response
func mockDeprovision(t *testing.T, api *testutil.MockHttpServer, environmentID string, response *ephemeralenvironments.DeprovisionEphemeralEnvironmentResponse) {
	api.ExpectRequest(t, "POST", fmt.Sprintf("/api/%s/environments/ephemeral/%s/deprovision", testSpaceID, environmentID)).
		RespondWith(response)
}

// Helper function to create test dependencies
func createTestDependencies(t *testing.T, api *testutil.MockHttpServer, envName string) (*cmd.Dependencies, *deprovisionEnvironment.DeprovisionEnvironmentOptions) {
	octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
	assert.NoError(t, err)

	space := spaces.NewSpace("Test Space")
	space.ID = testSpaceID

	deps := &cmd.Dependencies{
		Client:   octopus,
		Space:    space,
		NoPrompt: true,
	}

	flags := &deprovisionEnvironment.DeprovisionEnvironmentFlags{
		Name: flag.New[string](deprovisionEnvironment.FlagName, false),
	}
	flags.Name.Value = envName

	command := &cobra.Command{}
	command.SetOut(new(bytes.Buffer))

	opts := &deprovisionEnvironment.DeprovisionEnvironmentOptions{
		DeprovisionEnvironmentFlags: flags,
		Dependencies:                deps,
		Command:                     command,
	}

	return deps, opts
}

// Integration tests using MockHttpServer
func TestDeprovisionEnvironment_Success(t *testing.T) {
	const envName = "test-env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"

	env := createTestEnvironment(envID, envName)
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, envName)

		buf := new(bytes.Buffer)
		opts.Command.SetOut(buf)

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.NoError(t, err)

		// Verify that the output contains expected messages
		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env})
	mockDeprovision(t, api, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_EnvironmentNotFound(t *testing.T) {
	const envName = "nonexistent-env"

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, envName)

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("no ephemeral environment found with the name '%s'", envName))

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_MultipleEnvironmentsWithoutExactMatch(t *testing.T) {
	const searchName = "test"
	env1 := createTestEnvironment("Environments-1", "test-env-1")
	env2 := createTestEnvironment("Environments-2", "test-env-2")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, searchName)

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("could not find an exact match of an ephemeral environment with the name '%s'", searchName))

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvByPartialName(t, api, searchName, []*ephemeralenvironments.EphemeralEnvironment{env1, env2})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_MultipleEnvironmentsWithExactMatch(t *testing.T) {
	const envName = "test-env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"

	env1 := createTestEnvironment(envID, envName)
	env2 := createTestEnvironment("Environments-2", "test-env-2")
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, envName)

		buf := new(bytes.Buffer)
		opts.Command.SetOut(buf)

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env1, env2})
	mockDeprovision(t, api, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_CaseInsensitiveMatch(t *testing.T) {
	const searchName = "test-env"
	const actualName = "Test-Env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"

	env := createTestEnvironment(envID, actualName)
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, searchName)

		buf := new(bytes.Buffer)
		opts.Command.SetOut(buf)

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)
	mockGetEnvByPartialName(t, api, searchName, []*ephemeralenvironments.EphemeralEnvironment{env})
	mockDeprovision(t, api, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_EmptyName(t *testing.T) {
	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, "")

		err := deprovisionEnvironment.DeprovisionEnvironmentRun(opts, opts.Command)
		assert.Error(t, err)
		assert.Equal(t, "environment name is required", err.Error())

		return nil
	})

	setupMockAPI(t, api)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionEnvironment_PromptForName(t *testing.T) {
	const envName = "test-env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"

	env1 := createTestEnvironment(envID, envName)
	env2 := createTestEnvironment("Environments-2", "another-env")
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api, qa := testutil.NewMockServerAndAsker()
	defer testutil.Close(api, qa)

	errReceiver := testutil.GoBegin(func() error {
		octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
		assert.NoError(t, err)

		space := spaces.NewSpace("Test Space")
		space.ID = testSpaceID

		// Create dependencies with NoPrompt: false and empty name to trigger prompting
		deps := &cmd.Dependencies{
			Client:   octopus,
			Space:    space,
			NoPrompt: false,
			Ask:      qa.AsAsker(),
		}

		flags := &deprovisionEnvironment.DeprovisionEnvironmentFlags{
			Name: flag.New[string](deprovisionEnvironment.FlagName, false),
		}
		// Leave Name.Value empty to trigger prompting

		opts := &deprovisionEnvironment.DeprovisionEnvironmentOptions{
			DeprovisionEnvironmentFlags: flags,
			Dependencies:                deps,
		}
		command := &cobra.Command{}
		command.SetOut(new(bytes.Buffer))

		// Override GetAllEphemeralEnvironments to return our test environments
		opts.GetAllEphemeralEnvironments = func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			return []*ephemeralenvironments.EphemeralEnvironment{env1, env2}, nil
		}

		buf := new(bytes.Buffer)
		command.SetOut(buf)

		err = deprovisionEnvironment.DeprovisionEnvironmentRun(opts, command)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)

	// Expect the selector to ask which environment to deprovision
	_ = qa.ExpectQuestion(t, &survey.Select{
		Message: "Please select the name of the environment you wish to deprovision",
		Options: []string{envName, "another-env"},
	}).AnswerWith(envName)

	// After selection, it will look up the environment by name
	mockGetEnvByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env1})
	mockDeprovision(t, api, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}
