package integration_test

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"strings"
	"testing"
)

const space1ID = "Spaces-1"

func deleteAllReleasesInProject(t *testing.T, apiClient *octopusApiClient.Client, project *projects.Project) {
	projectReleases, err := apiClient.Projects.GetReleases(project)
	if !testutil.AssertSuccess(t, err) {
		return
	}
	for _, r := range projectReleases {
		_ = apiClient.Releases.DeleteByID(r.ID)
	}
}

func TestReleaseCreateBasics(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)

	project := fx.Project // alias for convenience

	dep, err := apiClient.DeploymentProcesses.Get(fx.Project, "")
	if !testutil.AssertSuccess(t, err) {
		return
	}
	dep.Steps = []*deployments.DeploymentStep{
		{
			Name:       fmt.Sprintf("step1-%s", runId),
			Properties: map[string]core.PropertyValue{"Octopus.Action.TargetRoles": core.NewPropertyValue("deploy", false)},
			Actions: []*deployments.DeploymentAction{
				{
					ActionType: "Octopus.Script",
					Name:       "Run a script",
					Properties: map[string]core.PropertyValue{
						"Octopus.Action.Script.ScriptBody": core.NewPropertyValue("echo 'hello'", false),
					},
				},
			},
		},
	}
	dep, err = apiClient.DeploymentProcesses.Update(dep)
	if !testutil.AssertSuccess(t, err) {
		return
	}

	// whilst the project already has a Default channel, we make an explicit one
	// so we can verify things aren't just silently using the default when we tell them not to
	customChannel := channels.NewChannel(fmt.Sprintf("channel-%s", runId), project.ID)
	customChannel, err = apiClient.Channels.Add(customChannel)
	if !testutil.AssertSuccess(t, err) {
		return
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Channels.DeleteByID(customChannel.ID)) })

	t.Run("create a release specifying project,channel,version", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--channel", customChannel.Name, "--version", "2.3.4")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

		projectReleases, err := apiClient.Projects.GetReleases(project)
		assert.Equal(t, 1, len(projectReleases))
		r1 := projectReleases[0]

		assert.Equal(t, project.ID, r1.ProjectID)
		assert.Equal(t, customChannel.ID, r1.ChannelID)
		assert.Equal(t, "2.3.4", r1.Version)

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Regexp(t, "Successfully created release version 2.3.4", stdOut) // unit tests check full text, we just want the basic confirmation
	})

	t.Run("create a release specifying project,channel - server allocates version", func(t *testing.T) {
		// create a phoney release so that when the server allocates the version for the next release it will be predictable
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--channel", customChannel.Name, "--version", "5.0.0")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}

		// the one we care about
		stdOut, stdErr, err = integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--channel", customChannel.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

		projectReleases, err := apiClient.Projects.GetReleases(project)
		assert.Equal(t, 2, len(projectReleases))
		r1 := projectReleases[0] // API returns newer releases first

		assert.Equal(t, project.ID, r1.ProjectID)
		assert.Equal(t, customChannel.ID, r1.ChannelID)
		assert.Equal(t, "5.0.1", r1.Version)

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Regexp(t, "Successfully created release version 5.0.1", stdOut) // unit tests check full text, we just want the basic confirmation
	})

	t.Run("create a release specifying project and version - server uses default channel", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--version", "6.0.0")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

		projectReleases, err := apiClient.Projects.GetReleases(project)
		assert.Equal(t, 1, len(projectReleases))
		r1 := projectReleases[0]

		assert.Equal(t, project.ID, r1.ProjectID)
		assert.Equal(t, fx.ProjectDefaultChannel.ID, r1.ChannelID)
		assert.Equal(t, "6.0.0", r1.Version)

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Regexp(t, "Successfully created release version 6.0.0", stdOut) // unit tests check full text, we just want the basic confirmation
	})

	t.Run("create a release specifying project - server uses default channel and allocates version", func(t *testing.T) {
		// create a phoney release so that when the server allocates the version for the next release it will be predictable
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--version", "7.0.0")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}

		// the one we care about
		stdOut, stdErr, err = integration.RunCli(space1ID, "release", "create", "--project", project.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

		projectReleases, err := apiClient.Projects.GetReleases(project)
		assert.Equal(t, 2, len(projectReleases))
		r1 := projectReleases[0]

		assert.Equal(t, project.ID, r1.ProjectID)
		assert.Equal(t, fx.ProjectDefaultChannel.ID, r1.ChannelID)
		assert.Equal(t, "7.0.1", r1.Version)

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Regexp(t, "Successfully created release version 7.0.1", stdOut) // unit tests check full text, we just want the basic confirmation
	})

	t.Run("cli returns an error if project is not specified", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create")

		if exiterr, ok := err.(*exec.ExitError); ok {
			assert.Equal(t, 1, exiterr.ExitCode())
		} else {
			assert.Fail(t, fmt.Sprintf("Expected ExitError from process, got %v", err))
		}

		assert.Equal(t, "\n", stdOut)
		assert.Equal(t, "project must be specified", stdErr)
	})
}

func TestReleaseListAndDelete(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)

	project := fx.Project // alias for convenience

	dep, err := apiClient.DeploymentProcesses.Get(fx.Project, "")
	if !testutil.AssertSuccess(t, err) {
		return
	}
	dep.Steps = []*deployments.DeploymentStep{
		{
			Name:       fmt.Sprintf("step1-%s", runId),
			Properties: map[string]core.PropertyValue{"Octopus.Action.TargetRoles": core.NewPropertyValue("deploy", false)},
			Actions: []*deployments.DeploymentAction{
				{
					ActionType: "Octopus.Script",
					Name:       "Run a script",
					Properties: map[string]core.PropertyValue{
						"Octopus.Action.Script.ScriptBody": core.NewPropertyValue("echo 'hello'", false),
					},
				},
			},
		},
	}
	dep, err = apiClient.DeploymentProcesses.Update(dep)
	if !testutil.AssertSuccess(t, err) {
		return
	}

	// create some releases so we can list them
	createReleaseCmd := releases.NewCreateReleaseCommandV1(space1ID, fx.Project.ID)
	for i := 0; i < 5; i++ {
		createReleaseCmd.ReleaseVersion = fmt.Sprintf("%d.0", i+1)
		_, err := releases.CreateReleaseV1(apiClient, createReleaseCmd)
		assert.Nil(t, err)
	}
	t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

	t.Run("list releases - basic", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "list", "--project", project.Name, "--output-format", "basic")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}

		assert.Equal(t, "5.0\n4.0\n3.0\n2.0\n1.0\n", stdOut)
		assert.Equal(t, "", stdErr)
	})

	t.Run("list releases - json", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "list", "--project", project.Name, "--output-format", "json")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}

		type x struct {
			Channel string
			Version string
		}

		parsed, err := testutil.ParseJsonStrict[[]x](strings.NewReader(stdOut))
		assert.Nil(t, err)
		assert.Equal(t, []x{
			{Channel: "Default", Version: "5.0"},
			{Channel: "Default", Version: "4.0"},
			{Channel: "Default", Version: "3.0"},
			{Channel: "Default", Version: "2.0"},
			{Channel: "Default", Version: "1.0"},
		}, parsed)
		assert.Equal(t, "", stdErr)
	})

	t.Run("list releases - default", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "list", "--project", project.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Equal(t, heredoc.Doc(`
			VERSION  CHANNEL
			5.0      Default
			4.0      Default
			3.0      Default
			2.0      Default
			1.0      Default
			`), stdOut)
		assert.Equal(t, "", stdErr)
	})

	t.Run("delete release", func(t *testing.T) {
		createReleaseCmd.ReleaseVersion = "DeleteMe5.0"
		createResponse, err := releases.CreateReleaseV1(apiClient, createReleaseCmd)
		require.Nil(t, err)

		// sanity check create worked so we can prove that deleting works
		resp, err := apiClient.Releases.GetByID(createResponse.ReleaseID)
		require.Nil(t, err)
		assert.Equal(t, "DeleteMe5.0", resp.Version)

		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "delete", "--project", project.Name, "--version", "DeleteMe5.0")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}

		assert.Regexp(t, "Success", stdOut)
		assert.Equal(t, "", stdErr)

		resp, err = apiClient.Releases.GetByID(createResponse.ReleaseID)
		assert.Nil(t, resp)

		apiErr, isCoreApiError := err.(*core.APIError)
		assert.True(t, isCoreApiError)
		assert.Equal(t, 404, apiErr.StatusCode)
		// the error struct contains an error message, but the server can/will change this over time, and we don't particularly care about it; 404 statuscode is the important bit
	})
}
