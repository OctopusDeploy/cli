package list_test

import (
	"bytes"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	octopusConstants "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// the shared root resource has no worker pool link; the target commands need it
var rootResource = newRootResourceWithWorkerPools()

func newRootResourceWithWorkerPools() *octopusApiClient.RootResource {
	root := testutil.NewRootResource()
	root.Links[octopusConstants.LinkWorkerPools] = octopusConstants.TestURIWorkerPools
	return root
}

const spaceID = "Spaces-1"

func TestDeploymentTargetListShowsDisabledState(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"table output has an IS DISABLED column", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			respondWithTargets(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "IS DISABLED")
			assert.Regexp(t, `web-server.*false`, stdOut.String())
			assert.Regexp(t, `db-server.*true`, stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"json output carries IsDisabled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "list", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			respondWithTargets(t, api)
			// the json mapper re-resolves the lookups per target
			for range 2 {
				api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
				api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/all").RespondWith([]*tenants.Tenant{})
			}

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), `"IsDisabled": false`)
			assert.Contains(t, stdOut.String(), `"IsDisabled": true`)
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

func respondWithTargets(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/machines?take=2147483647").
		RespondWith(resources.Resources[*machines.DeploymentTarget]{Items: []*machines.DeploymentTarget{
			fixtures.NewDeploymentTarget(spaceID, "Machines-100", "web-server", false),
			fixtures.NewDeploymentTarget(spaceID, "Machines-200", "db-server", true),
		}})
	api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
	api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/all").RespondWith([]*tenants.Tenant{})
	api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})
}

var development = fixtures.NewEnvironment(spaceID, "Environments-1", "Development")
