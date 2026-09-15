package view_test

import (
	"bytes"
	"net/url"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	octopusConstants "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// the shared root resource has no infrastructure links; the worker commands need them
var rootResource = newRootResourceWithInfrastructureLinks()

func newRootResourceWithInfrastructureLinks() *octopusApiClient.RootResource {
	root := testutil.NewRootResource()
	root.Links[octopusConstants.LinkWorkers] = octopusConstants.TestURIWorkers
	root.Links[octopusConstants.LinkWorkerPools] = octopusConstants.TestURIWorkerPools
	return root
}

const spaceID = "Spaces-1"

func newWorker(workerID string, name string, isDisabled bool) *machines.Worker {
	endpoint := machines.NewListeningTentacleEndpoint(
		&url.URL{Scheme: "https", Host: "worker:10933"}, "0123456789ABCDEF0123456789ABCDEF01234567")
	endpoint.TentacleVersionDetails = machines.NewTentacleVersionDetails("8.1.0", false, false, false)

	worker := machines.NewWorker(name, endpoint)
	worker.ID = workerID
	worker.SpaceID = spaceID
	worker.IsDisabled = isDisabled
	return worker
}

func TestWorkerViewShowsDisabledState(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"basic output reports a disabled worker", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "view", "Workers-100", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			respondWithWorker(t, api, true)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Disabled: true")
			assert.Equal(t, "", stdErr.String())
		}},

		{"basic output reports an enabled worker", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "view", "Workers-100", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			respondWithWorker(t, api, false)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Disabled: false")
			assert.Equal(t, "", stdErr.String())
		}},

		{"json output carries IsDisabled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "view", "Workers-100", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			respondWithWorker(t, api, true)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), `"IsDisabled": true`)
			assert.Equal(t, "", stdErr.String())
		}},

		{"table output has an IS DISABLED column", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "view", "Workers-100", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			respondWithWorker(t, api, true)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "IS DISABLED")
			assert.Regexp(t, `build-worker.*true`, stdOut.String())
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

func respondWithWorker(t *testing.T, api *testutil.MockHttpServer, isDisabled bool) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/workers/Workers-100").
		RespondWith(newWorker("Workers-100", "build-worker", isDisabled))
	api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})
}
