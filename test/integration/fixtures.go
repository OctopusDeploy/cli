package integration

import (
	"fmt"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

// contains shared setup code for creating integration tests dependencies;
// e.g. creating a project group, project, etc.
// Be very careful about what you promote as a shared text fixture here.
// Guidance: You should avoid doing things to shared fixture data that need "putting back".
// Ideally you'll use this stuff to get off the ground, make some tweaks to the data,
// then run your block of tests and let it all tear down at the end.
// If you find yourself wanting to make tweaks and then "put things back" then you probably
// should make your own specific thing in your test, rather than tweaking this.

type CommonProjectFixture struct {
	ProjectGroup          *projectgroups.ProjectGroup
	Lifecycle             *lifecycles.Lifecycle
	Project               *projects.Project
	ProjectDefaultChannel *channels.Channel
}

func CreateCommonProject(t *testing.T, apiClient *octopusApiClient.Client, runId uuid.UUID) (*CommonProjectFixture, error) {
	// pre-requisites
	projectGroup := projectgroups.NewProjectGroup(fmt.Sprintf("pg-%s", runId))
	projectGroup, err := apiClient.ProjectGroups.Add(projectGroup)
	if !testutil.AssertSuccess(t, err) {
		return nil, err
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.ProjectGroups.DeleteByID(projectGroup.ID)) })

	lifecycle := lifecycles.NewLifecycle(fmt.Sprintf("lifecycle-%s", runId))
	lifecycle, err = apiClient.Lifecycles.Add(lifecycle)
	if !testutil.AssertSuccess(t, err) {
		return nil, err
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Lifecycles.DeleteByID(lifecycle.ID)) })

	// The project creates its own Default channel, using the specified lifecycle
	project := projects.NewProject(fmt.Sprintf("project-%s", runId), lifecycle.ID, projectGroup.ID)
	project, err = apiClient.Projects.Add(project)
	if !testutil.AssertSuccess(t, err) {
		return nil, err
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Projects.DeleteByID(project.ID)) })

	projectChannels, err := apiClient.Projects.GetChannels(project)
	testutil.RequireSuccess(t, err)
	assert.Equal(t, 1, len(projectChannels))
	projectDefaultChannel := projectChannels[0]

	return &CommonProjectFixture{
		ProjectGroup:          projectGroup,
		Lifecycle:             lifecycle,
		Project:               project,
		ProjectDefaultChannel: projectDefaultChannel,
	}, nil
}
