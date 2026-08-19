package integration_test

import (
	"fmt"
	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"testing"
	"time"
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
		assert.Regexp(t, "2.3.4", stdOut) // unit tests check full text, we just want the basic confirmation
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
		assert.Regexp(t, "5.0.1", stdOut) // unit tests check full text, we just want the basic confirmation
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
		assert.Regexp(t, "6.0.0", stdOut) // unit tests check full text, we just want the basic confirmation
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
		assert.Regexp(t, "7.0.1", stdOut) // unit tests check full text, we just want the basic confirmation
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
	createReleaseCmd := releases.NewCreateReleaseCommandV1(space1ID, fx.Project.GetName())
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

func createEnvironment(t *testing.T, apiClient *octopusApiClient.Client, name string) *environments.Environment {
	environment, err := apiClient.Environments.Add(environments.NewEnvironment(name))
	if !testutil.AssertSuccess(t, err) {
		return nil
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Environments.DeleteByID(environment.GetID())) })
	return environment
}

func createCloudRegionTarget(t *testing.T, apiClient *octopusApiClient.Client, name string, environmentID string) *machines.DeploymentTarget {
	target, err := apiClient.Machines.Add(machines.NewDeploymentTarget(name, machines.NewCloudRegionEndpoint(), []string{environmentID}, []string{"deploy"}))
	if !testutil.AssertSuccess(t, err) {
		return nil
	}
	t.Cleanup(func() { assert.Nil(t, apiClient.Machines.DeleteByID(target.GetID())) })
	return target
}

// allowDeploymentsTo replaces the fixture lifecycle's phases with a single phase for the
// given environment, so releases in the project can be deployed to it.
func allowDeploymentsTo(t *testing.T, apiClient *octopusApiClient.Client, lifecycle *lifecycles.Lifecycle, environmentID string) bool {
	phase := lifecycles.NewPhase("phase1")
	phase.OptionalDeploymentTargets = []string{environmentID}
	lifecycle.Phases = []*lifecycles.Phase{phase}
	updated, err := apiClient.Lifecycles.Update(lifecycle)
	if !testutil.AssertSuccess(t, err) {
		return false
	}
	t.Cleanup(func() {
		updated.Phases = nil
		_, err := apiClient.Lifecycles.Update(updated)
		assert.Nil(t, err)
	})
	return true
}

// waitForTaskToComplete blocks until the deployment's server task finishes; the project cannot be
// deleted while it is still running. Whether it succeeded is not this test's concern.
func waitForTaskToComplete(t *testing.T, apiClient *octopusApiClient.Client, taskID string) {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		found, err := apiClient.Tasks.Get(tasks.TasksQuery{IDs: []string{taskID}})
		if !testutil.AssertSuccess(t, err) {
			return
		}
		if len(found.Items) == 1 && found.Items[0].IsCompleted != nil && *found.Items[0].IsCompleted {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Errorf("timed out waiting for task %s to complete", taskID)
}

// scriptStep builds a single inline script step. With no target roles it runs on the server, so
// the project is deployable without any deployment targets.
func scriptStep(name string, targetRoles string) *deployments.DeploymentStep {
	stepProperties := map[string]core.PropertyValue{}
	action := &deployments.DeploymentAction{
		ActionType: "Octopus.Script",
		Name:       name,
		Properties: map[string]core.PropertyValue{
			"Octopus.Action.Script.ScriptBody": core.NewPropertyValue("echo 'hello'", false),
		},
	}
	if targetRoles != "" {
		stepProperties["Octopus.Action.TargetRoles"] = core.NewPropertyValue(targetRoles, false)
	} else {
		action.Properties["Octopus.Action.RunOnServer"] = core.NewPropertyValue("true", false)
	}
	return &deployments.DeploymentStep{Name: name, Properties: stepProperties, Actions: []*deployments.DeploymentAction{action}}
}

// packageStep builds a single package step. The server rejects a package on an inline script,
// so a release that needs a package version has to go through this step type.
func packageStep(name string, targetRoles string, packageID string) *deployments.DeploymentStep {
	return &deployments.DeploymentStep{
		Name:       name,
		Properties: map[string]core.PropertyValue{"Octopus.Action.TargetRoles": core.NewPropertyValue(targetRoles, false)},
		Actions: []*deployments.DeploymentAction{
			{
				ActionType: "Octopus.TentaclePackage",
				Name:       name,
				Properties: map[string]core.PropertyValue{},
				Packages: []*packages.PackageReference{
					{
						PackageID:           packageID,
						FeedID:              "feeds-builtin",
						AcquisitionLocation: "Server",
						Properties:          map[string]string{"SelectionMode": "immediate"},
					},
				},
			},
		},
	}
}

func setDeploymentProcess(t *testing.T, apiClient *octopusApiClient.Client, project *projects.Project, step *deployments.DeploymentStep) bool {
	deploymentProcess, err := apiClient.DeploymentProcesses.Get(project, "")
	if !testutil.AssertSuccess(t, err) {
		return false
	}
	deploymentProcess.Steps = []*deployments.DeploymentStep{step}
	_, err = apiClient.DeploymentProcesses.Update(deploymentProcess)
	return testutil.AssertSuccess(t, err)
}

func onlyReleaseInProject(t *testing.T, apiClient *octopusApiClient.Client, project *projects.Project) *releases.Release {
	projectReleases, err := apiClient.Projects.GetReleases(project)
	if !testutil.AssertSuccess(t, err) {
		return nil
	}
	require.Equal(t, 1, len(projectReleases))
	return projectReleases[0]
}

func onlyDeploymentOfRelease(t *testing.T, apiClient *octopusApiClient.Client, release *releases.Release) *deployments.Deployment {
	releaseDeployments, err := apiClient.Deployments.GetDeployments(release)
	if !testutil.AssertSuccess(t, err) {
		return nil
	}
	require.Equal(t, 1, len(releaseDeployments.Items))
	return releaseDeployments.Items[0]
}

// The executions API reports an unknown release version poorly - as a null reference error on the
// servers in issue #294, and as a bare "was not found" on current ones - so the CLI resolves the
// version up front and says what it looked for.
func TestReleaseDeployUnknownVersion(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)
	project := fx.Project

	environment := createEnvironment(t, apiClient, fmt.Sprintf("env-%s", runId))
	if environment == nil {
		return
	}

	t.Run("the API does not answer with a usable release", func(t *testing.T) {
		release, err := releases.GetReleaseInProject(apiClient, space1ID, project.GetID(), "9.9.9")
		assert.True(t, err != nil || release == nil || release.GetID() == "")
	})

	t.Run("deploy names the version it could not find", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "deploy", "--project", project.Name, "--version", "9.9.9", "--environment", environment.Name)
		assert.Error(t, err)
		assert.Contains(t, stdErr, fmt.Sprintf("cannot find a release with version '9.9.9' in project '%s'", project.Name))
		assert.NotContains(t, stdErr, "Object reference not set")
	})

	t.Run("deploy reports that latest is not an alias", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "deploy", "--project", project.Name, "--version", "latest", "--environment", environment.Name)
		assert.Error(t, err)
		assert.Contains(t, stdErr, "'latest' is not a supported alias")
		assert.NotContains(t, stdErr, "Object reference not set")
	})
}

// A package with no version in its feed fails the release with no indication of which package is
// at fault, so the CLI diagnoses the failure and names them. See issue #426.
func TestReleaseCreateMissingPackageVersion(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)
	project := fx.Project

	stepName := fmt.Sprintf("step-%s", runId)
	packageID := fmt.Sprintf("package-%s", runId)
	if !setDeploymentProcess(t, apiClient, project, packageStep(stepName, "deploy", packageID)) {
		return
	}

	t.Run("create names the package that has no version", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name)
		assert.Error(t, err)
		assert.Contains(t, stdErr, "no version could be found for the following packages")
		assert.Contains(t, stdErr, packageID)
		assert.Contains(t, stdErr, stepName)
		assert.NotContains(t, stdErr, "Object reference not set")
	})
}

// The executions API matches channels and environments by name only, so the CLI resolves IDs
// before sending them. See issue #250.
func TestReleaseCreateAndDeployByID(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)
	project := fx.Project

	environment := createEnvironment(t, apiClient, fmt.Sprintf("env-%s", runId))
	if environment == nil {
		return
	}
	if !allowDeploymentsTo(t, apiClient, fx.Lifecycle, environment.GetID()) {
		return
	}
	if !setDeploymentProcess(t, apiClient, project, scriptStep(fmt.Sprintf("step-%s", runId), "")) {
		return
	}
	t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

	t.Run("create accepts a channel ID", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--channel", fx.ProjectDefaultChannel.GetID(), "--version", "1.0.0")
		if !testutil.AssertSuccess(t, err, stdErr) {
			return
		}
		release := onlyReleaseInProject(t, apiClient, project)
		if release == nil {
			return
		}
		assert.Equal(t, fx.ProjectDefaultChannel.GetID(), release.ChannelID)
	})

	t.Run("deploy accepts an environment ID", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "deploy", "--project", project.Name, "--version", "1.0.0", "--environment", environment.GetID())
		if !testutil.AssertSuccess(t, err, stdErr) {
			return
		}
		release := onlyReleaseInProject(t, apiClient, project)
		if release == nil {
			return
		}
		deployment := onlyDeploymentOfRelease(t, apiClient, release)
		if deployment == nil {
			return
		}
		assert.Equal(t, environment.GetID(), deployment.EnvironmentID)
		waitForTaskToComplete(t, apiClient, deployment.TaskID)
	})
}

// Comma-separated values are split before they reach the executions API, which otherwise reports
// the whole string as one unknown target. See issue #556.
func TestReleaseDeployCommaSeparatedTargets(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	fx, err := integration.CreateCommonProject(t, apiClient, runId)
	testutil.RequireSuccess(t, err)
	project := fx.Project

	environment := createEnvironment(t, apiClient, fmt.Sprintf("env-%s", runId))
	if environment == nil {
		return
	}
	if !allowDeploymentsTo(t, apiClient, fx.Lifecycle, environment.GetID()) {
		return
	}
	if !setDeploymentProcess(t, apiClient, project, scriptStep(fmt.Sprintf("step-%s", runId), "deploy")) {
		return
	}

	targetA := createCloudRegionTarget(t, apiClient, fmt.Sprintf("target-a-%s", runId), environment.GetID())
	targetB := createCloudRegionTarget(t, apiClient, fmt.Sprintf("target-b-%s", runId), environment.GetID())
	if targetA == nil || targetB == nil {
		return
	}
	t.Cleanup(func() { deleteAllReleasesInProject(t, apiClient, project) })

	_, stdErr, err := integration.RunCli(space1ID, "release", "create", "--project", project.Name, "--version", "1.0.0")
	if !testutil.AssertSuccess(t, err, stdErr) {
		return
	}

	t.Run("deploy splits a comma-separated target list", func(t *testing.T) {
		_, stdErr, err := integration.RunCli(space1ID, "release", "deploy", "--project", project.Name, "--version", "1.0.0", "--environment", environment.Name, "--deployment-target", fmt.Sprintf("%s,%s", targetA.Name, targetB.Name))
		if !testutil.AssertSuccess(t, err, stdErr) {
			return
		}
		release := onlyReleaseInProject(t, apiClient, project)
		if release == nil {
			return
		}
		deployment := onlyDeploymentOfRelease(t, apiClient, release)
		if deployment == nil {
			return
		}
		assert.ElementsMatch(t, []string{targetA.GetID(), targetB.GetID()}, deployment.SpecificMachineIDs)
		waitForTaskToComplete(t, apiClient, deployment.TaskID)
	})
}
