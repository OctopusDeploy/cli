package worker_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/worker"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerHasEnableAndDisableCommands(t *testing.T) {
	cmd := worker.NewCmdWorker(testutil.NewMockFactory(testutil.NewMockHttpServer()))

	assert.NotNil(t, findCommand(cmd, "enable"))
	assert.NotNil(t, findCommand(cmd, "disable"))
}

func TestEveryWorkerCreateCommandSupportsDisabled(t *testing.T) {
	root := worker.NewCmdWorker(testutil.NewMockFactory(testutil.NewMockHttpServer()))

	workerTypes := []string{"listening-tentacle", "ssh"}
	for _, workerType := range workerTypes {
		t.Run(workerType, func(t *testing.T) {
			typeCmd := findCommand(root, workerType)
			require.NotNil(t, typeCmd)

			createCmd := findCommand(typeCmd, "create")
			require.NotNil(t, createCmd)

			disabled := createCmd.Flags().Lookup(machinescommon.FlagDisabled)
			require.NotNil(t, disabled)
			assert.Equal(t, "false", disabled.DefValue)
		})
	}
}

func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
