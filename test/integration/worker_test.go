package integration_test

import (
	"fmt"
	"testing"

	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listening tentacles are the cheapest worker to create for real: the server
// records the endpoint without checking it can be reached
func createListeningTentacleWorker(t *testing.T, apiClient *octopusApiClient.Client, poolName string, name string, extraArgs ...string) *machines.Worker {
	args := append([]string{
		"worker", "listening-tentacle", "create",
		"--name", name,
		"--worker-pool", poolName,
		"--machine-policy", "Default Machine Policy",
		"--thumbprint", "0123456789ABCDEF0123456789ABCDEF01234567",
		"--url", fmt.Sprintf("https://%s.invalid:10933", name),
	}, extraArgs...)

	stdOut, stdErr, err := integration.RunCli("Default", args...)
	if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
		return nil
	}

	worker, err := apiClient.Workers.GetByIdentifier(name)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Workers.DeleteByID(worker.GetID())) })
	return worker
}

// settings a toggle must not disturb. HealthStatus, Status and StatusSummary are
// excluded on purpose: the server derives them from IsDisabled.
type workerSettings struct {
	Name            string
	WorkerPoolIDs   []string
	MachinePolicyID string
	Thumbprint      string
	URI             string
	EndpointType    string
}

func workerSettingsOf(worker *machines.Worker) workerSettings {
	return workerSettings{
		Name:            worker.Name,
		WorkerPoolIDs:   worker.WorkerPoolIDs,
		MachinePolicyID: worker.MachinePolicyID,
		Thumbprint:      worker.Thumbprint,
		URI:             worker.URI,
		EndpointType:    fmt.Sprintf("%T", worker.Endpoint),
	}
}

func TestWorkerEnableDisable(t *testing.T) {
	runId := uuid.New()
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	pools, err := apiClient.WorkerPools.GetAll()
	testutil.RequireSuccess(t, err)

	// dynamic pools provision their own workers, so only a static pool will take one
	poolName := ""
	for _, pool := range pools {
		if pool.CanAddWorkers {
			poolName = pool.Name
			break
		}
	}
	require.NotEmpty(t, poolName, "the space needs a worker pool that accepts workers")

	t.Run("create --disabled", func(t *testing.T) {
		worker := createListeningTentacleWorker(t, apiClient, poolName, fmt.Sprintf("wkr-disabled-%s", runId), "--disabled")
		require.NotNil(t, worker)
		assert.True(t, worker.IsDisabled)
	})

	t.Run("create without --disabled", func(t *testing.T) {
		worker := createListeningTentacleWorker(t, apiClient, poolName, fmt.Sprintf("wkr-enabled-%s", runId))
		require.NotNil(t, worker)
		assert.False(t, worker.IsDisabled)
	})

	t.Run("enable and disable change nothing else", func(t *testing.T) {
		worker := createListeningTentacleWorker(t, apiClient, poolName, fmt.Sprintf("wkr-toggle-%s", runId), "--disabled")
		require.NotNil(t, worker)
		before := workerSettingsOf(worker)

		stdOut, stdErr, err := integration.RunCli("Default", "worker", "enable", worker.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Successfully enabled worker '%s'", worker.Name))

		enabled, err := apiClient.Workers.GetByIdentifier(worker.GetID())
		testutil.RequireSuccess(t, err)
		assert.False(t, enabled.IsDisabled)

		// the update is a read-modify-write of the whole worker, so the rest of
		// its settings have to survive the round trip
		assert.Equal(t, before, workerSettingsOf(enabled))

		// and back again, by ID this time
		stdOut, stdErr, err = integration.RunCli("Default", "worker", "disable", worker.GetID())
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Successfully disabled worker '%s'", worker.Name))

		disabled, err := apiClient.Workers.GetByIdentifier(worker.GetID())
		testutil.RequireSuccess(t, err)
		assert.True(t, disabled.IsDisabled)
		assert.Equal(t, before, workerSettingsOf(disabled))
	})

	t.Run("already in the requested state", func(t *testing.T) {
		worker := createListeningTentacleWorker(t, apiClient, poolName, fmt.Sprintf("wkr-noop-%s", runId))
		require.NotNil(t, worker)

		stdOut, stdErr, err := integration.RunCli("Default", "worker", "enable", worker.Name)
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("Worker '%s' (%s) is already enabled.", worker.Name, worker.GetID()))

		unchanged, err := apiClient.Workers.GetByIdentifier(worker.GetID())
		testutil.RequireSuccess(t, err)
		assert.False(t, unchanged.IsDisabled)
		assert.Equal(t, worker.ModifiedOn, unchanged.ModifiedOn, "no update should have been sent")
	})

	t.Run("errors", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			args     []string
			expected string
		}{
			{"unknown name", []string{"enable", "no-such-worker"}, "cannot find worker with name or ID of 'no-such-worker'"},
			{"no identifier without prompting", []string{"disable"}, "worker identifier is required but was not provided"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := append([]string{"worker"}, tc.args...)
				stdOut, stdErr, err := integration.RunCli("Default", args...)
				assert.Error(t, err, stdOut)
				assert.Contains(t, stdOut+stdErr, tc.expected)
			})
		}

		t.Run("a deployment target is not a worker", func(t *testing.T) {
			targets, err := apiClient.Machines.Get(machines.MachinesQuery{Take: 1})
			testutil.RequireSuccess(t, err)
			if len(targets.Items) == 0 {
				t.Skip("no deployment targets in this space")
			}
			targetID := targets.Items[0].GetID()

			stdOut, stdErr, err := integration.RunCli("Default", "worker", "enable", targetID)
			assert.Error(t, err, stdOut)
			assert.Contains(t, stdOut+stdErr, fmt.Sprintf("cannot find worker with name or ID of '%s'", targetID))
		})
	})
}
