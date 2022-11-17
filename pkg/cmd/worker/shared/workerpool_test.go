package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForWorkerPool_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewWorkerPoolFlags()
	flags.WorkerPools.Value = []string{"Head lifeguard"}

	opts := shared.NewWorkerPoolOptions(&cmd.Dependencies{Ask: asker})
	err := shared.PromptForWorkerPools(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForWorkerPool_NoFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Select the worker pools to assign the worker to", "", []string{"Groundskeeper", "Swim instructor"}, []string{"Groundskeeper", "Swim instructor"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewWorkerPoolFlags()

	opts := shared.NewWorkerPoolOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllWorkerPoolsCallback = func() ([]*workerpools.WorkerPoolListResult, error) {
		poolWorker1 := &workerpools.WorkerPoolListResult{
			ID:             "WorkerPools-1",
			Name:           "Groundskeeper",
			WorkerPoolType: workerpools.WorkerPoolTypeStatic,
			CanAddWorkers:  true,
		}
		poolWorker2 := &workerpools.WorkerPoolListResult{
			ID:             "WorkerPools-2",
			Name:           "Swim instructor",
			WorkerPoolType: workerpools.WorkerPoolTypeStatic,
			CanAddWorkers:  true,
		}
		return []*workerpools.WorkerPoolListResult{poolWorker1, poolWorker2}, nil
	}
	err := shared.PromptForWorkerPools(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, []string{"Groundskeeper", "Swim instructor"}, flags.WorkerPools.Value)
}
