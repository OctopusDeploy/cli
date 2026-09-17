package root_test

import (
	"bytes"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runVersion executes a command that needs neither a server nor prompting, so the only
// thing under test is the shell validation in PersistentPreRunE.
func runVersion(t *testing.T, args ...string) (string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactory(testutil.NewMockHttpServer()), nil, nil)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(append([]string{"version"}, args...))

	err := rootCmd.Execute()
	return stderr.String(), err
}

// the shell is a config setting and an env var, never a flag; Editor and ShowOctopus are
// the same shape. The persistent flag set is deliberately small, and every entry in it
// shows up in each command's help and in the generated docs.
func TestRoot_ThereIsNoShellFlag(t *testing.T) {
	_, err := runVersion(t, "--shell", "bash")
	require.Error(t, err)
	assert.EqualError(t, err, "unknown flag: --shell")
}

// an invalid OCTOPUS_SHELL is warned about rather than rejected; it is set once and
// applies to every command afterwards, so failing would lock the user out of the CLI
// entirely, including the config command needed to fix it.
func TestRoot_InvalidShellEnvIsWarnedAboutbutNotFatal(t *testing.T) {
	t.Setenv(constants.EnvOctopusShell, "fish")

	stderr, err := runVersion(t)
	assert.NoError(t, err)
	assert.Contains(t, stderr, "Warning: ignoring OCTOPUS_SHELL: the provided value fish is not a valid shell")
}

func TestRoot_ValidShellEnvIsSilent(t *testing.T) {
	t.Setenv(constants.EnvOctopusShell, "pwsh")

	stderr, err := runVersion(t)
	assert.NoError(t, err)
	assert.Equal(t, "", stderr)
}
