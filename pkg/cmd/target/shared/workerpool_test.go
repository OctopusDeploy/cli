package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForWorkerPool_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetWorkerPoolFlags()
	flags.WorkerPool.Value = "Head lifeguard"

	opts := shared.NewCreateTargetWorkerPoolOptions(&cmd.Dependencies{Ask: asker})
	err := shared.PromptForWorkerPool(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForWorkerPool_NoFlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Will this worker use the default worker pool?", "", false, true),
		testutil.NewSelectPrompt("Select the worker pool to use", "", []string{"Groundskeeper", "Swim instructor"}, "Groundskeeper"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetWorkerPoolFlags()

	opts := shared.NewCreateTargetWorkerPoolOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllWorkerPoolsCallback = func() ([]*workerpools.WorkerPoolListResult, error) {
		poolWorker1 := &workerpools.WorkerPoolListResult{
			ID:             "WorkerPools-1",
			Name:           "Groundskeeper",
			WorkerPoolType: workerpools.WorkerPoolTypeStatic,
		}
		poolWorker2 := &workerpools.WorkerPoolListResult{
			ID:             "WorkerPools-2",
			Name:           "Swim instructor",
			WorkerPoolType: workerpools.WorkerPoolTypeDynamic,
		}
		return []*workerpools.WorkerPoolListResult{poolWorker1, poolWorker2}, nil
	}
	err := shared.PromptForWorkerPool(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Groundskeeper", flags.WorkerPool.Value)
}

func TestPromptForWorkerPool_UseDefault(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Will this worker use the default worker pool?", "", true, true),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetWorkerPoolFlags()

	opts := shared.NewCreateTargetWorkerPoolOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForWorkerPool(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Empty(t, flags.WorkerPool.Value)
}
