package dryrun_test

import (
	"bytes"
	"net/http"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/dryrun"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGuardRoundTripper(t *testing.T) {
	tests := []struct {
		method  string
		blocked bool
	}{
		{http.MethodGet, false},
		{http.MethodHead, false},
		{http.MethodOptions, false},
		{http.MethodPost, true},
		{http.MethodPut, true},
		{http.MethodPatch, true},
		{http.MethodDelete, true},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			next := &testutil.RecordingRoundTripper{}
			guard := dryrun.NewGuardRoundTripper(next)

			request, err := http.NewRequest(test.method, "http://server/api/Spaces-1/releases/Releases-1", nil)
			assert.Nil(t, err)

			response, err := guard.RoundTrip(request)

			if test.blocked {
				assert.Nil(t, response)
				assert.EqualError(t, err, "dry run blocked a "+test.method+" request to /api/Spaces-1/releases/Releases-1; this command does not fully support --dry-run, please raise an issue")
				assert.Empty(t, next.Requests, "the request must not reach the server")
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, response)
				assert.Len(t, next.Requests, 1)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	t.Run("false when the command doesn't declare the flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: "thing"}
		assert.False(t, dryrun.IsEnabled(cmd))
	})

	t.Run("false when the flag is declared but not set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "thing"}
		value := false
		dryrun.AddFlag(cmd.Flags(), &value)
		assert.False(t, dryrun.IsEnabled(cmd))
	})

	t.Run("true when the flag is set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "thing"}
		value := false
		dryrun.AddFlag(cmd.Flags(), &value)
		assert.Nil(t, cmd.Flags().Set(constants.FlagDryRun, "true"))
		assert.True(t, dryrun.IsEnabled(cmd))
		assert.True(t, value)
	})
}

// --dry-run must never be silently accepted by a command that hasn't implemented it,
// or the caller would believe a mutation was skipped when it wasn't.
func TestUnsupportedCommandRejectsDryRun(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	api := testutil.NewMockHttpServer()
	space1 := fixtures.NewSpace("Spaces-1", "Default Space")

	rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"release", "list", "--project", "Fire Project", "--dry-run"})

	_, err := rootCmd.ExecuteC()
	assert.EqualError(t, err, "unknown flag: --dry-run")
}
