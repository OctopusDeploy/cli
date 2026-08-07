package view_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/target/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	octopusConstants "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shared root resource has no infrastructure links; the target commands need them.
var rootResource = newRootResourceWithInfrastructureLinks()

func newRootResourceWithInfrastructureLinks() *octopusApiClient.RootResource {
	root := testutil.NewRootResource()
	root.Links[octopusConstants.LinkMachines] = octopusConstants.TestURIMachines
	root.Links[octopusConstants.LinkWorkerPools] = octopusConstants.TestURIWorkerPools

	return root
}

// Sent verbatim: the SDK has no type that can represent an ECS endpoint.
var ecsTargetResponse = json.RawMessage(`{
  "Id": "Machines-1041",
  "Name": "aws ecs",
  "SpaceId": "Spaces-1",
  "EnvironmentIds": ["Environments-1"],
  "Roles": ["ecs"],
  "TenantIds": [],
  "TenantTags": [],
  "HealthStatus": "Healthy",
  "StatusSummary": "This machine was successfully health checked.",
  "IsDisabled": false,
  "Endpoint": {
    "CommunicationStyle": "AwsEcsCluster",
    "DefaultWorkerPoolId": "WorkerPools-1",
    "ClusterName": "repro-604-cluster",
    "Region": "ap-southeast-2",
    "AccountId": "",
    "UseInstanceRole": true,
    "AssumeRole": false,
    "AssumedRoleArn": null,
    "AssumedRoleSession": null,
    "AssumeRoleSessionDurationSeconds": null,
    "AssumeRoleExternalId": null,
    "Id": null,
    "LastModifiedOn": null,
    "LastModifiedBy": null,
    "Links": {}
  }
}`)

// Regression test for #604.
func TestViewEcsTarget_DoesNotPanic(t *testing.T) {
	api, rootCmd, stdOut := setupViewCommand(t)

	cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
		defer api.Close()
		rootCmd.SetArgs([]string{"view", "Machines-1041", "-f", "basic"})
		return rootCmd.ExecuteC()
	})

	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-1041").RespondWith(ecsTargetResponse)
	expectEnvironmentRequest(t, api)
	expectWorkerPoolRequest(t, api)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/all").RespondWith(resources.Resources[*tenants.Tenant]{})
	// The basic formatter resolves the same names again, minus tenants.
	expectEnvironmentRequest(t, api)
	expectWorkerPoolRequest(t, api)

	_, err := testutil.ReceivePair(cmdReceiver)
	assert.NoError(t, err)

	output := stdOut.String()
	assert.Contains(t, output, "aws ecs")
	assert.Contains(t, output, "Healthy")
	assert.Contains(t, output, "Environments: Development")
	assert.Contains(t, output, "Roles: ecs")
	assert.Contains(t, output, "Type: Unknown")
}

func TestViewEcsTarget_Json(t *testing.T) {
	api, rootCmd, stdOut := setupViewCommand(t)

	cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
		defer api.Close()
		rootCmd.SetArgs([]string{"view", "Machines-1041", "-f", "json"})
		return rootCmd.ExecuteC()
	})

	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-1041").RespondWith(ecsTargetResponse)
	expectEnvironmentRequest(t, api)
	expectWorkerPoolRequest(t, api)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/all").RespondWith(resources.Resources[*tenants.Tenant]{})

	_, err := testutil.ReceivePair(cmdReceiver)
	assert.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(stdOut.Bytes(), &result))
	assert.Equal(t, "aws ecs", result["Name"])
	assert.Equal(t, "Machines-1041", result["Id"])
	assert.Equal(t, []any{"Development"}, result["Environments"])
	assert.Equal(t, "", result["CommunicationStyle"])
}

func setupViewCommand(t *testing.T) (*testutil.MockHttpServer, *cobra.Command, *bytes.Buffer) {
	t.Helper()

	api := testutil.NewMockHttpServer()
	space := spaces.NewSpace("Default")
	space.ID = "Spaces-1"

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}

	rootCmd := &cobra.Command{Use: "deployment-target"}
	rootCmd.PersistentFlags().StringP(constants.FlagOutputFormat, "f", constants.OutputFormatTable, "")
	rootCmd.AddCommand(view.NewCmdView(testutil.NewMockFactoryWithSpace(api, space)))
	rootCmd.SetOut(stdOut)
	rootCmd.SetErr(stdErr)

	return api, rootCmd, stdOut
}

func expectEnvironmentRequest(t *testing.T, api *testutil.MockHttpServer) {
	t.Helper()

	development := environments.NewEnvironment("Development")
	development.ID = "Environments-1"

	api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})
}

func expectWorkerPoolRequest(t *testing.T, api *testutil.MockHttpServer) {
	t.Helper()

	api.ExpectRequest(t, "GET", "/api/Spaces-1/workerpools/all").RespondWith([]*workerpools.WorkerPoolListResult{})
}
