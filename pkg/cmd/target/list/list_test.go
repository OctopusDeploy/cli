package list

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeTargetType_KnownStyle(t *testing.T) {
	target := machines.NewDeploymentTarget("web-01", machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint"), nil, nil)

	assert.Equal(t, "Listening Tentacle", describeTargetType(target))
}

func TestDescribeTargetType_EndpointMissing(t *testing.T) {
	target := &machines.DeploymentTarget{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"Id": "Machines-1041",
		"Name": "aws ecs",
		"Endpoint": { "CommunicationStyle": "AwsEcsCluster", "ClusterName": "repro-604-cluster" }
	}`), target))

	assert.NotPanics(t, func() {
		assert.Equal(t, "Unknown", describeTargetType(target))
	})
}
