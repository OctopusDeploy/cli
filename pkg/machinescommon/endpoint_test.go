package machinescommon_test

import (
	"net/url"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommunicationStyle_KnownEndpoint(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")

	assert.Equal(t, "TentaclePassive", machinescommon.GetCommunicationStyle(endpoint))
}

// The SDK leaves the endpoint nil for a communication style it does not model.
func TestGetCommunicationStyle_NilEndpoint(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Equal(t, "", machinescommon.GetCommunicationStyle(nil))
	})
}

// A typed nil pointer in the interface, which a plain != nil check would miss.
func TestGetCommunicationStyle_TypedNilEndpoint(t *testing.T) {
	var endpoint *machines.ListeningTentacleEndpoint

	assert.NotPanics(t, func() {
		assert.Equal(t, "", machinescommon.GetCommunicationStyle(endpoint))
	})
}

func TestFormatUri(t *testing.T) {
	assert.Equal(t, "https://tentacle:10933", machinescommon.FormatUri(&url.URL{Scheme: "https", Host: "tentacle:10933"}))
	assert.Equal(t, "Unknown", machinescommon.FormatUri(nil))
}

func TestFormatTentacleVersion(t *testing.T) {
	assert.Equal(t, "6.3.1", machinescommon.FormatTentacleVersion(&machines.TentacleVersionDetails{Version: "6.3.1"}))
	assert.Equal(t, "Unknown", machinescommon.FormatTentacleVersion(nil))
}

func TestEndpointAs_MatchingType(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")

	typedEndpoint, err := machinescommon.EndpointAs[*machines.ListeningTentacleEndpoint](endpoint, machinescommon.WorkerNoun, "Listening Tentacle")

	require.NoError(t, err)
	assert.Equal(t, endpoint, typedEndpoint)
}

func TestEndpointAs_MismatchedType(t *testing.T) {
	endpoint := machines.NewCloudRegionEndpoint()

	_, targetErr := machinescommon.EndpointAs[*machines.ListeningTentacleEndpoint](endpoint, machinescommon.DeploymentTargetNoun, "Listening Tentacle")
	_, workerErr := machinescommon.EndpointAs[*machines.SSHEndpoint](endpoint, machinescommon.WorkerNoun, "SSH")

	assert.EqualError(t, targetErr, "this deployment target is not of type Listening Tentacle")
	assert.EqualError(t, workerErr, "this worker is not of type SSH")
}

func TestEndpointAs_NilEndpoint(t *testing.T) {
	assert.NotPanics(t, func() {
		_, err := machinescommon.EndpointAs[*machines.ListeningTentacleEndpoint](nil, machinescommon.WorkerNoun, "Listening Tentacle")
		assert.Error(t, err)
	})
}

func TestDescribeCommunicationStyle(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")
	agent := machines.NewKubernetesTentacleEndpoint(&url.URL{Scheme: "https", Host: "agent:10933"}, "thumbprint", false, "Polling", "default")

	assert.Equal(t, "Listening Tentacle", machinescommon.DescribeCommunicationStyle(endpoint, machinescommon.CommunicationStyleToDescriptionMap))
	// A style with no friendly name of its own is still worth showing verbatim.
	assert.Equal(t, "KubernetesTentacle", machinescommon.DescribeCommunicationStyle(agent, machinescommon.CommunicationStyleToDescriptionMap))
	assert.Equal(t, "Unknown", machinescommon.DescribeCommunicationStyle(nil, machinescommon.CommunicationStyleToDescriptionMap))
}
