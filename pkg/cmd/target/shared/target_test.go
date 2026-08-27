package shared_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An AWS ECS target as the API returns it, trimmed to the fields the CLI reads.
// Captured from /api/Spaces-1/machines on a server with an ECS target configured.
const ecsTargetJson = `{
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
}`

// If this fails because the SDK learned to deserialise "AwsEcsCluster", the
// unknown-type fallbacks below can start reporting real detail.
func TestEcsTargetDeserialisesWithoutAnEndpoint(t *testing.T) {
	target := parseTarget(t, ecsTargetJson)

	assert.True(t, machines.IsNil(target.Endpoint), "expected the SDK to leave the endpoint nil for an AwsEcsCluster target")
}

func TestGetEndpointDetails_EndpointMissing(t *testing.T) {
	target := parseTarget(t, ecsTargetJson)

	var details map[string]string
	assert.NotPanics(t, func() {
		details = shared.GetEndpointDetails(target)
	})
	assert.Empty(t, details)
}

func TestGetEndpointDetails_KnownEndpoint(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")
	endpoint.TentacleVersionDetails = &machines.TentacleVersionDetails{Version: "6.3.1"}
	target := machines.NewDeploymentTarget("web-01", endpoint, []string{"Environments-1"}, []string{"web"})

	details := shared.GetEndpointDetails(target)

	assert.Equal(t, "https://tentacle:10933", details["URI"])
	assert.Equal(t, "6.3.1", details["Tentacle version"])
}

func TestGetEndpointDetails_TentacleNeverHealthChecked(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")
	target := machines.NewDeploymentTarget("web-01", endpoint, []string{"Environments-1"}, []string{"web"})

	var details map[string]string
	assert.NotPanics(t, func() {
		details = shared.GetEndpointDetails(target)
	})
	assert.Equal(t, "Unknown", details["Tentacle version"])
	assert.Equal(t, "https://tentacle:10933", details["URI"])
}

func TestResolveDefaultWorkerPool_EndpointMissing(t *testing.T) {
	target := parseTarget(t, ecsTargetJson)

	assert.NotPanics(t, func() {
		assert.Equal(t, "N/A", shared.ResolveDefaultWorkerPool(target, map[string]string{}, "None"))
	})
}

func parseTarget(t *testing.T, payload string) *machines.DeploymentTarget {
	t.Helper()

	target := &machines.DeploymentTarget{}
	require.NoError(t, json.Unmarshal([]byte(payload), target))

	return target
}
