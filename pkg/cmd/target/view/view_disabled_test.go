package view_test

import (
	"bytes"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

const spaceID = "Spaces-1"

var development = fixtures.NewEnvironment(spaceID, "Environments-1", "Development")

func TestDeploymentTargetViewShowsDisabledState(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"basic output reports a disabled target", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "view", "Machines-100", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			respondWithTarget(t, api, true)
			// the basic renderer re-resolves the lookups it needs
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Disabled: true")
			assert.Equal(t, "", stdErr.String())
		}},

		{"basic output reports an enabled target", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "view", "Machines-100", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			respondWithTarget(t, api, false)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Disabled: false")
			assert.Equal(t, "", stdErr.String())
		}},

		{"json output carries IsDisabled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "view", "Machines-100", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			respondWithTarget(t, api, true)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), `"IsDisabled": true`)
			assert.Equal(t, "", stdErr.String())
		}},

		{"table output has an IS DISABLED column", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "view", "Machines-100", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			respondWithTarget(t, api, true)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "IS DISABLED")
			assert.Regexp(t, `web-server.*true`, stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, qa, rootCmd, stdout, stderr)
		})
	}
}

func respondWithTarget(t *testing.T, api *testutil.MockHttpServer, isDisabled bool) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-100").
		RespondWith(fixtures.NewDeploymentTarget(spaceID, "Machines-100", "web-server", isDisabled))
	api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
	api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})
	api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/all").RespondWith([]*tenants.Tenant{})
}
