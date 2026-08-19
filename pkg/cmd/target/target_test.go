package target_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/target"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentTargetHasEnableAndDisableCommands(t *testing.T) {
	cmd := target.NewCmdDeploymentTarget(testutil.NewMockFactory(testutil.NewMockHttpServer()))

	assert.NotNil(t, findCommand(cmd, "enable"))
	assert.NotNil(t, findCommand(cmd, "disable"))
}

func TestEveryTargetCreateCommandSupportsDisabled(t *testing.T) {
	root := target.NewCmdDeploymentTarget(testutil.NewMockFactory(testutil.NewMockHttpServer()))

	targetTypes := []string{"azure-web-app", "cloud-region", "kubernetes", "listening-tentacle", "ssh"}
	for _, targetType := range targetTypes {
		t.Run(targetType, func(t *testing.T) {
			typeCmd := findCommand(root, targetType)
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
