package view

import (
	"net/url"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
)

func TestGetWorkerTypeDisplayName(t *testing.T) {
	assert.Equal(t, "Listening Tentacle", getWorkerTypeDisplayName("TentaclePassive"))
	assert.Equal(t, "SomeFutureWorkerType", getWorkerTypeDisplayName("SomeFutureWorkerType"))
	assert.Equal(t, "Unknown", getWorkerTypeDisplayName(""))
}

func TestGetEndpointDetails_Tentacle(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")
	endpoint.TentacleVersionDetails = &machines.TentacleVersionDetails{Version: "6.3.1"}

	details := getEndpointDetails(machines.NewWorker("worker-01", endpoint))

	assert.Equal(t, "https://tentacle:10933", details["URI"])
	assert.Equal(t, "6.3.1", details["Tentacle version"])
}

func TestGetEndpointDetails_TentacleNeverHealthChecked(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")

	var details map[string]string
	assert.NotPanics(t, func() {
		details = getEndpointDetails(machines.NewWorker("worker-01", endpoint))
	})
	assert.Equal(t, "Unknown", details["Tentacle version"])
}

func TestGetEndpointDetails_EndpointMissing(t *testing.T) {
	var details map[string]string
	assert.NotPanics(t, func() {
		details = getEndpointDetails(machines.NewWorker("worker-01", nil))
	})
	assert.Empty(t, details)
}
