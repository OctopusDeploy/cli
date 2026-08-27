package list

import (
	"net/url"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
)

func TestDescribeWorker_KnownStyle(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")

	assert.Equal(t, "TentaclePassive", describeWorkerType(endpoint))
	assert.Equal(t, "Listening Tentacle", describeWorkerStyle(endpoint))
}

func TestDescribeWorker_EndpointMissing(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Equal(t, "Unknown", describeWorkerType(nil))
		assert.Equal(t, "Unknown", describeWorkerStyle(nil))
	})
}

// A style the SDK models without a display name of its own still has to report
// something.
func TestDescribeWorker_UnnamedStyle(t *testing.T) {
	endpoint := machines.NewKubernetesTentacleEndpoint(&url.URL{Scheme: "https", Host: "agent:10933"}, "thumbprint", false, "Polling", "default")

	assert.Equal(t, "KubernetesTentacle", describeWorkerType(endpoint))
	assert.Equal(t, "KubernetesTentacle", describeWorkerStyle(endpoint))
}

func TestGetEndpointUriAndVersion_TentacleNeverHealthChecked(t *testing.T) {
	endpoint := machines.NewPollingTentacleEndpoint(&url.URL{Scheme: "poll", Host: "tentacle"}, "thumbprint")

	assert.NotPanics(t, func() {
		assert.Equal(t, "poll://tentacle", getEndpointUri(endpoint))
		assert.Equal(t, "Unknown", getVersion(endpoint))
	})
}

func TestGetEndpointUriAndVersion_EndpointMissing(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Equal(t, "", getEndpointUri(nil))
		assert.Equal(t, "", getVersion(nil))
		assert.Equal(t, "", getRuntimeArchitecture(nil))
	})
}
