package integration_test

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"math"
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

func TestReleaseCreate(t *testing.T) {
	runId := uuid.New()

	// setup; we need a project in a project and a channel in order to release things
	apiClient, err := integration.GetApiClient(space1ID)
	if !testutil.EnsureSuccess(t, err) {
		return
	}

	projectGroup := projectgroups.NewProjectGroup(fmt.Sprintf("pg-%s", runId))
	projectGroup, err = apiClient.ProjectGroups.Add(projectGroup)
	if !testutil.EnsureSuccess(t, err) {
		return
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.ProjectGroups.DeleteByID(projectGroup.ID)) })

	lifecycle := lifecycles.NewLifecycle(fmt.Sprintf("lifecycle-%s", runId))
	lifecycle, err = apiClient.Lifecycles.Add(lifecycle)
	if !testutil.EnsureSuccess(t, err) {
		return
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Lifecycles.DeleteByID(lifecycle.ID)) })

	project := projects.NewProject(fmt.Sprintf("project-%s", runId), lifecycle.ID, projectGroup.ID)
	project, err = apiClient.Projects.Add(project)
	if !testutil.EnsureSuccess(t, err) {
		return
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Projects.DeleteByID(project.ID)) })

	dep, err := apiClient.DeploymentProcesses.Get(project, "")
	if !testutil.EnsureSuccess(t, err) {
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
	if !testutil.EnsureSuccess(t, err) {
		return
	}

	channel := channels.NewChannel(fmt.Sprintf("channel-%s", runId), project.ID)
	channel, err = apiClient.Channels.Add(channel)
	if !testutil.EnsureSuccess(t, err) {
		return
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Channels.DeleteByID(channel.ID)) })

	t.Run("create a release specifying project + channel + version", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--channel", channel.Name, "--version", "2.3.4")
		if !testutil.EnsureSuccess(t, err, stdOut, stdErr) {
			return
		}

		t.Cleanup(func() {
			releasesPage, err := apiClient.Releases.Get(releases.ReleasesQuery{Take: math.MaxInt})
			if !testutil.EnsureSuccess(t, err) {
				return
			}
			for _, r := range releasesPage.Items {
				_ = apiClient.Releases.DeleteByID(r.ID)
			}
		})

		assert.Equal(t, heredoc.Docf(`
			Project project-%s
			Channel channel-%s
			Version 2.3.4
			`, runId, runId), stdOut)

		projectReleases, err := apiClient.Projects.GetReleases(project)
		assert.Equal(t, 1, len(projectReleases))
		r1 := projectReleases[0]

		assert.Equal(t, project.ID, r1.ProjectID)
		assert.Equal(t, channel.ID, r1.ChannelID)
		assert.Equal(t, "2.3.4", r1.Version)
	})
}
