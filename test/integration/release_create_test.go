package integration_test

import (
	"fmt"
	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

// things to test (interactive mode):
// NOTE the question flow does the matching here:
//   case-insensitive match on channel name
//   no partial match on channel name
//   case-insensitive match on project name
//   no partial match on project name

// things to test (automation mode):
// NOTE the question flow does not run here, should it?
//   case-insensitive match on channel name
//   no partial match on channel name
//   case-insensitive match on project name
//   no partial match on project name

// DESIGN QUESTION:
// In automation mode, if a user specifies "foo project" and the actual thing is "Foo PROJECT", should the
// command do a pass over the options first, and replace things with their correctly-cased versions before
// feeding into the executor, or should the executor do that?
//
// The nicest thing would be if the executor could just be a blind pass-through into the server, however
// because the octopus server doesn't support things like
// matching a project based on exact name, we have to do at least SOME client side filtering first.
// The executions API may be an exception to this rule, but in general, it holds.

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

func TestReleaseCreate(t *testing.T) {
	runId := uuid.New()

	// pre-requisites
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	projectGroup := projectgroups.NewProjectGroup(fmt.Sprintf("pg-%s", runId))
	projectGroup, err = apiClient.ProjectGroups.Add(projectGroup)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.ProjectGroups.DeleteByID(projectGroup.ID)) })

	lifecycle := lifecycles.NewLifecycle(fmt.Sprintf("lifecycle-%s", runId))
	lifecycle, err = apiClient.Lifecycles.Add(lifecycle)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Lifecycles.DeleteByID(lifecycle.ID)) })

	// The project creates its own Default channel, using the specified lifecycle
	project := projects.NewProject(fmt.Sprintf("project-%s", runId), lifecycle.ID, projectGroup.ID)
	project, err = apiClient.Projects.Add(project)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Projects.DeleteByID(project.ID)) })

	projectChannels, err := apiClient.Projects.GetChannels(project)
	testutil.RequireSuccess(t, err)
	assert.Equal(t, 1, len(projectChannels))
	projectDefaultChannel := projectChannels[0]

	dep, err := apiClient.DeploymentProcesses.Get(project, "")
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
		assert.Equal(t, fmt.Sprintf("Successfully created release version 2.3.4 (%s) using channel %s (%s)\n", r1.ID, customChannel.Name, customChannel.ID), stdOut)
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
		assert.Equal(t, fmt.Sprintf("Successfully created release version 5.0.1 (%s) using channel %s (%s)\n", r1.ID, customChannel.Name, customChannel.ID), stdOut)
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
		assert.Equal(t, projectDefaultChannel.ID, r1.ChannelID)
		assert.Equal(t, "6.0.0", r1.Version)

		// TODO should the CLI output that it's using the Default channel, and possibly what the name of that channel is?

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Equal(t, fmt.Sprintf("Successfully created release version 6.0.0 (%s) using channel Default (%s)\n", r1.ID, projectDefaultChannel.ID), stdOut)
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
		assert.Equal(t, projectDefaultChannel.ID, r1.ChannelID)
		assert.Equal(t, "7.0.1", r1.Version)

		// TODO should the CLI output that it's using the Default channel, and possibly what the name of that channel is?

		// assert CLI output *after* we've gone to the server and looked up what we expect the release ID to be.
		assert.Equal(t, fmt.Sprintf("Successfully created release version 7.0.1 (%s) using channel Default (%s)\n", r1.ID, projectDefaultChannel.ID), stdOut)
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
