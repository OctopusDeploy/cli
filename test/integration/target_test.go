package integration_test

import (
	"fmt"
	"testing"

	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cloud regions are the cheapest target to create for real: no endpoint
// credentials and no connectivity for the server to check
func createCloudRegion(t *testing.T, apiClient *octopusApiClient.Client, envName string, name string, extraArgs ...string) *machines.DeploymentTarget {
	args := append([]string{
		"deployment-target", "cloud-region", "create",
		"--name", name, "--environment", envName, "--role", "target-tests",
	}, extraArgs...)

	stdOut, stdErr, err := integration.RunCli("Default", args...)
	if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
		return nil
	}

	target, err := apiClient.Machines.GetByIdentifier(name)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Machines.DeleteByID(target.GetID())) })
	return target
}

// a second endpoint type, to check the server honours IsDisabled on more than
// just cloud regions. Needs an explicit machine policy under --no-prompt.
func createListeningTentacle(t *testing.T, apiClient *octopusApiClient.Client, envName string, name string, thumbprint string, extraArgs ...string) *machines.DeploymentTarget {
	args := append([]string{
		"deployment-target", "listening-tentacle", "create",
		"--name", name, "--environment", envName, "--role", "target-tests",
		"--machine-policy", "Default Machine Policy",
		"--thumbprint", thumbprint,
		"--url", fmt.Sprintf("https://%s.invalid:10933", name),
	}, extraArgs...)

	stdOut, stdErr, err := integration.RunCli("Default", args...)
	if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
		return nil
	}

	target, err := apiClient.Machines.GetByIdentifier(name)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Machines.DeleteByID(target.GetID())) })
	return target
}

// settings a toggle must not disturb. HealthStatus, Status and StatusSummary are
// excluded on purpose: the server derives them from IsDisabled.
type targetSettings struct {
	Name                   string
	EnvironmentIDs         []string
	Roles                  []string
	TenantedDeploymentMode string
	TenantIDs              []string
	TenantTags             []string
	MachinePolicyID        string
	Thumbprint             string
	URI                    string
	EndpointType           string
}

func settingsOf(target *machines.DeploymentTarget) targetSettings {
	return targetSettings{
		Name:                   target.Name,
		EnvironmentIDs:         target.EnvironmentIDs,
		Roles:                  target.Roles,
		TenantedDeploymentMode: string(target.TenantedDeploymentMode),
		TenantIDs:              target.TenantIDs,
		TenantTags:             target.TenantTags,
		MachinePolicyID:        target.MachinePolicyID,
		Thumbprint:             target.Thumbprint,
		URI:                    target.URI,
		EndpointType:           fmt.Sprintf("%T", target.Endpoint),
	}
}

func TestDeploymentTargetEnableDisable(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	env, err := apiClient.Environments.Add(environments.NewEnvironment(fmt.Sprintf("tgtenv-%s", runId)))
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Environments.DeleteByID(env.GetID())) })

	t.Run("create --disabled", func(t *testing.T) {
		target := createCloudRegion(t, apiClient, env.Name, fmt.Sprintf("tgt-disabled-%s", runId), "--disabled")
		require.NotNil(t, target)
		assert.True(t, target.IsDisabled)
	})

	t.Run("create without --disabled", func(t *testing.T) {
		target := createCloudRegion(t, apiClient, env.Name, fmt.Sprintf("tgt-enabled-%s", runId))
		require.NotNil(t, target)
		assert.False(t, target.IsDisabled)
	})

	t.Run("create --disabled on a listening tentacle", func(t *testing.T) {
		target := createListeningTentacle(t, apiClient, env.Name,
			fmt.Sprintf("tgt-lt-disabled-%s", runId), "0123456789ABCDEF0123456789ABCDEF01234567", "--disabled")
		require.NotNil(t, target)
		assert.True(t, target.IsDisabled)
	})

	t.Run("enable and disable change nothing else", func(t *testing.T) {
		target := createCloudRegion(t, apiClient, env.Name, fmt.Sprintf("tgt-toggle-%s", runId), "--disabled")
		require.NotNil(t, target)
		before := settingsOf(target)

		stdOut, stdErr, err := integration.RunCli("Default", "deployment-target", "enable", target.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Successfully enabled deployment target '%s'", target.Name))

		enabled, err := apiClient.Machines.GetByIdentifier(target.GetID())
		testutil.RequireSuccess(t, err)
		assert.False(t, enabled.IsDisabled)

		// the update is a read-modify-write of the whole target, so the rest of
		// its settings have to survive the round trip
		assert.Equal(t, before, settingsOf(enabled))

		// and back again, by ID this time
		stdOut, stdErr, err = integration.RunCli("Default", "deployment-target", "disable", target.GetID())
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Successfully disabled deployment target '%s'", target.Name))

		disabled, err := apiClient.Machines.GetByIdentifier(target.GetID())
		testutil.RequireSuccess(t, err)
		assert.True(t, disabled.IsDisabled)
		assert.Equal(t, before, settingsOf(disabled))
	})

	t.Run("already in the requested state", func(t *testing.T) {
		target := createCloudRegion(t, apiClient, env.Name, fmt.Sprintf("tgt-noop-%s", runId))
		require.NotNil(t, target)

		stdOut, stdErr, err := integration.RunCli("Default", "deployment-target", "enable", target.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Deployment target '%s' (%s) is already enabled.", target.Name, target.GetID()))

		unchanged, err := apiClient.Machines.GetByIdentifier(target.GetID())
		testutil.RequireSuccess(t, err)
		assert.False(t, unchanged.IsDisabled)
		assert.Equal(t, target.ModifiedOn, unchanged.ModifiedOn, "no update should have been sent")
	})

	t.Run("errors", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			args     []string
			expected string
		}{
			{"unknown name", []string{"enable", "no-such-target"}, "cannot find machine with the name or ID of 'no-such-target'"},
			{"no identifier without prompting", []string{"disable"}, "deployment target identifier is required but was not provided"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := append([]string{"deployment-target"}, tc.args...)
				stdOut, stdErr, err := integration.RunCli("Default", args...)
				assert.Error(t, err, stdOut)
				assert.Contains(t, stdOut+stdErr, tc.expected)
			})
		}

		t.Run("a worker is not a deployment target", func(t *testing.T) {
			workers, err := apiClient.Workers.Get(machines.WorkersQuery{Take: 1})
			testutil.RequireSuccess(t, err)
			if len(workers.Items) == 0 {
				t.Skip("no workers in this space")
			}
			workerID := workers.Items[0].GetID()

			stdOut, stdErr, err := integration.RunCli("Default", "deployment-target", "enable", workerID)
			assert.Error(t, err, stdOut)
			assert.Contains(t, stdOut+stdErr, fmt.Sprintf("cannot find machine with the name or ID of '%s'", workerID))
		})
	})
}
