package install_test

import (
	"context"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Octopus's registration API takes an environment slug or ID, not the display
// name someone naturally types, so whatever --environment was given is
// resolved before it is sent.
func TestResolveWithoutPrompting_EnvironmentNamesBecomeSlugs(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := completedFlags()
	flags.Environments.Value = []string{"Production", "development"}
	opts := newOptions(t, flags, asker)

	require.NoError(t, opts.ResolveWithoutPrompting(context.Background()))
	assert.Equal(t, []string{"production", "development"}, flags.Environments.Value)
}

func TestResolveWithoutPrompting_RejectsAnUnknownEnvironment(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := completedFlags()
	flags.Environments.Value = []string{"Staging"}
	opts := newOptions(t, flags, asker)

	assert.ErrorContains(t, opts.ResolveWithoutPrompting(context.Background()), `"Staging"`)
}

func completedFlags() *install.InstallFlags {
	flags := install.NewInstallFlags()
	flags.Name.Value = "Production"
	flags.Environments.Value = []string{"production"}
	flags.ArgoCDToken.Value = "eyJhbGciOiJIUzI1NiJ9.token"
	return flags
}
