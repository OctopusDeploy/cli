package shared_test

import (
	"bytes"
	"net/url"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusConstants "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// the per-type worker views all render through shared.ViewRun, so one of them is
// enough to cover the Disabled row it adds
func TestPerTypeWorkerViewShowsDisabledState(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	root := testutil.NewRootResource()
	root.Links[octopusConstants.LinkWorkers] = octopusConstants.TestURIWorkers
	root.Links[octopusConstants.LinkWorkerPools] = octopusConstants.TestURIWorkerPools

	for _, tc := range []struct {
		name       string
		isDisabled bool
		expected   string
	}{
		{"disabled worker", true, "true"},
		{"enabled worker", false, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "listening-tentacle", "view", "Workers-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(root)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(root)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workers/Workers-100").
				RespondWith(newTestWorker(spaceID, "Workers-100", "build-worker", tc.isDisabled))
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Regexp(t, `Disabled\s+`+tc.expected, stdout.String())
			assert.Equal(t, "", stderr.String())
		})
	}
}

func newTestWorker(spaceID string, workerID string, name string, isDisabled bool) *machines.Worker {
	endpoint := machines.NewListeningTentacleEndpoint(
		&url.URL{Scheme: "https", Host: "worker:10933"}, "0123456789ABCDEF0123456789ABCDEF01234567")
	endpoint.TentacleVersionDetails = machines.NewTentacleVersionDetails("8.1.0", false, false, false)

	worker := machines.NewWorker(name, endpoint)
	worker.ID = workerID
	worker.SpaceID = spaceID
	worker.IsDisabled = isDisabled
	return worker
}
