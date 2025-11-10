package deprovision_project_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	deprovisionProject "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-project"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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

// Helper function to create test ephemeral environment
func createTestEnvironment(id, name string) *ephemeralenvironments.EphemeralEnvironment {
	env := ephemeralenvironments.NewEphemeralEnvironment(name, "Environments-1", testSpaceID)
	env.ID = id
	return env
}

// Helper function to create test project
func createProject(projectName string, projectID string) *projects.Project {
	project := projects.NewProject(projectName, "Lifecycles-1", "ProjectGroups-1")
	project.ID = projectID
	return project
}

// Helper function to create a deprovision response
func createDeprovisionResponse(runbookRunID, taskID string) *ephemeralenvironments.DeprovisionEphemeralEnvironmentProjectResponse {
	return &ephemeralenvironments.DeprovisionEphemeralEnvironmentProjectResponse{
		DeprovisioningRun: ephemeralenvironments.DeprovisioningRunbookRun{
			RunbookRunID: runbookRunID,
			TaskId:       taskID,
		},
	}
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

// Helper function to mock the Deprovision API response
func mockDeprovision(t *testing.T, api *testutil.MockHttpServer, projectID string, environmentID string, response *ephemeralenvironments.DeprovisionEphemeralEnvironmentProjectResponse) {
	api.ExpectRequest(t, "POST", fmt.Sprintf("/api/%s/projects/%s/environments/ephemeral/%s/deprovision", testSpaceID, projectID, environmentID)).
		RespondWith(response)
}

// Helper function to mock the Deprovision API response
func mockGetProjectByName(t *testing.T, api *testutil.MockHttpServer, name string, response *projects.Project) {
	api.ExpectRequest(t, "GET", fmt.Sprintf("/api/%s/projects?partialName=%s", testSpaceID, url.PathEscape(name))).
		RespondWith(&resources.Resources[*projects.Project]{
			Items:        []*projects.Project{response},
			PagedResults: resources.PagedResults{TotalResults: 1},
		})
}

// Helper function to create test dependencies
func createTestDependencies(t *testing.T, api *testutil.MockHttpServer, envName string, projectName string) (*cmd.Dependencies, *deprovisionProject.DeprovisionProjectOptions) {
	octopus, err := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, testSpaceID)
	assert.NoError(t, err)

	space := spaces.NewSpace("Test Space")
	space.ID = testSpaceID

	deps := &cmd.Dependencies{
		Client:   octopus,
		Space:    space,
		NoPrompt: true,
	}

	flags := &deprovisionProject.DeprovisionProjectFlags{
		Name:    flag.New[string](deprovisionProject.FlagName, false),
		Project: flag.New[string](deprovisionProject.FlagProject, false),
	}
	flags.Name.Value = envName
	flags.Project.Value = projectName

	command := &cobra.Command{}
	command.SetOut(new(bytes.Buffer))

	opts := &deprovisionProject.DeprovisionProjectOptions{
		DeprovisionProjectFlags: flags,
		Dependencies:            deps,
		Cmd:                     command,
	}

	return deps, opts
}

// Integration tests using MockHttpServer
func TestDeprovisionProject_Success(t *testing.T) {
	const envName = "test-env"
	const projectID = "Projects-1"
	const projectName = "Test Project"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"

	env := createTestEnvironment(envID, envName)
	project := createProject(projectName, projectID)
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	assert.NotNil(t, deprovisionResponse)
	api := testutil.NewMockHttpServer()

	errReceiver := testutil.GoBegin(func() error {
		defer api.Close()
		_, opts := createTestDependencies(t, api, envName, projectName)

		buf := new(bytes.Buffer)
		opts.Cmd.SetOut(buf)

		// Capture console output to verify the printed messages
		err := deprovisionProject.DeprovisionEphemeralEnvironmentProject(opts.Cmd, opts)
		assert.NoError(t, err)

		output := buf.String()
		// Verify that the output contains expected messages
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	assert.NotNil(t, errReceiver)

	setupMockAPI(t, api)
	mockGetProjectByName(t, api, projectName, project)
	mockGetByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env})
	mockDeprovision(t, api, projectID, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionProject_EnvironmentNotFound(t *testing.T) {
	const envName = "nonexistent-env"
	const projectID = "Projects-1"
	const projectName = "Test Project"

	project := createProject(projectName, projectID)

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, envName, projectName)

		err := deprovisionProject.DeprovisionEphemeralEnvironmentProject(opts.Cmd, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("no ephemeral environment found with the name '%s'", envName))

		return nil
	})

	setupMockAPI(t, api)
	mockGetProjectByName(t, api, projectName, project)
	mockGetByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{})

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionProject_EmptyName(t *testing.T) {

	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, "", "project")

		err := deprovisionProject.DeprovisionEphemeralEnvironmentProject(opts.Cmd, opts)
		assert.Error(t, err)
		assert.Equal(t, "environment name is required", err.Error())

		return nil
	})

	setupMockAPI(t, api)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionProject_EmptyProjectName(t *testing.T) {
	api := testutil.NewMockHttpServer()
	defer api.Close()

	errReceiver := testutil.GoBegin(func() error {
		_, opts := createTestDependencies(t, api, "environment", "")

		err := deprovisionProject.DeprovisionEphemeralEnvironmentProject(opts.Cmd, opts)
		assert.Error(t, err)
		assert.Equal(t, "project name is required", err.Error())

		return nil
	})

	setupMockAPI(t, api)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionProject_PromptForName(t *testing.T) {
	const envName = "test-env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"
	const projectName = "Test Project"
	const projectID = "Projects-1"

	env1 := createTestEnvironment(envID, envName)
	env2 := createTestEnvironment("Environments-2", "another-env")
	project := createProject(projectName, projectID)
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api, qa := testutil.NewMockServerAndAsker()

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
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

		flags := &deprovisionProject.DeprovisionProjectFlags{
			Name:    flag.New[string](deprovisionProject.FlagName, false),
			Project: flag.New[string](deprovisionProject.FlagProject, false),
		}

		flags.Project.Value = projectName

		opts := &deprovisionProject.DeprovisionProjectOptions{
			DeprovisionProjectFlags: flags,
			Dependencies:            deps,
		}
		command := &cobra.Command{}
		command.SetOut(new(bytes.Buffer))
		opts.Cmd = command

		// Override GetAllEphemeralEnvironments to return our test environments
		opts.GetAllEphemeralEnvironments = func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			return []*ephemeralenvironments.EphemeralEnvironment{env1, env2}, nil
		}

		opts.GetConfiguredProjectsCallback = func() ([]*projects.Project, error) {
			return []*projects.Project{project}, nil
		}

		buf := new(bytes.Buffer)
		command.SetOut(buf)

		err = deprovisionProject.DeprovisionEphemeralEnvironmentProject(command, opts)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)

	// Expect the selector to ask which environment to deprovision
	_ = qa.ExpectQuestion(t, &survey.Select{
		Message: "Please select the name of the environment you wish to deprovision:",
		Options: []string{envName, "another-env"},
	}).AnswerWith(envName)

	// After selection, it will look up the environment by name
	mockGetProjectByName(t, api, projectName, project)
	mockGetByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env1})
	mockDeprovision(t, api, projectID, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}

func TestDeprovisionProject_PromptForProjectName(t *testing.T) {
	const envName = "test-env"
	const envID = "Environments-1"
	const runbookRunID = "RunbookRuns-1"
	const taskID = "ServerTasks-1"
	const projectName = "Test Project"
	const projectID = "Projects-1"

	env1 := createTestEnvironment(envID, envName)
	env2 := createTestEnvironment("Environments-2", "another-env")
	project := createProject(projectName, projectID)
	project2 := createProject("Another Project", "Projects-2")
	deprovisionResponse := createDeprovisionResponse(runbookRunID, taskID)

	api, qa := testutil.NewMockServerAndAsker()

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
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

		flags := &deprovisionProject.DeprovisionProjectFlags{
			Name:    flag.New[string](deprovisionProject.FlagName, false),
			Project: flag.New[string](deprovisionProject.FlagProject, false),
		}

		flags.Name.Value = envName
		// flags.Project.Value = projectName

		opts := &deprovisionProject.DeprovisionProjectOptions{
			DeprovisionProjectFlags: flags,
			Dependencies:            deps,
		}
		command := &cobra.Command{}
		command.SetOut(new(bytes.Buffer))
		opts.Cmd = command

		// Override GetAllEphemeralEnvironments to return our test environments
		opts.GetAllEphemeralEnvironments = func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			return []*ephemeralenvironments.EphemeralEnvironment{env1, env2}, nil
		}

		opts.GetConfiguredProjectsCallback = func() ([]*projects.Project, error) {
			return []*projects.Project{project, project2}, nil
		}

		buf := new(bytes.Buffer)
		command.SetOut(buf)

		err = deprovisionProject.DeprovisionEphemeralEnvironmentProject(command, opts)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, runbookRunID)
		assert.Contains(t, output, taskID)

		return nil
	})

	setupMockAPI(t, api)

	// Expect the selector to ask which project to deprovision
	_ = qa.ExpectQuestion(t, &survey.Select{
		Message: "Select a project:",
		Options: []string{projectName, "Another Project"},
	}).AnswerWith(projectName)

	// After selection, it will look up the environment by name
	mockGetProjectByName(t, api, projectName, project)
	mockGetByPartialName(t, api, envName, []*ephemeralenvironments.EphemeralEnvironment{env1})
	mockDeprovision(t, api, projectID, envID, deprovisionResponse)

	err := <-errReceiver
	assert.NoError(t, err)
}
