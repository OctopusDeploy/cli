package shared_test

import (
	"net/url"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointAs_MatchingType(t *testing.T) {
	endpoint := machines.NewListeningTentacleEndpoint(&url.URL{Scheme: "https", Host: "tentacle:10933"}, "thumbprint")

	typedEndpoint, err := shared.EndpointAs[*machines.ListeningTentacleEndpoint](endpoint, "Listening Tentacle")

	require.NoError(t, err)
	assert.Equal(t, endpoint, typedEndpoint)
}

func TestEndpointAs_MismatchedType(t *testing.T) {
	endpoint := machines.NewCloudRegionEndpoint()

	_, err := shared.EndpointAs[*machines.ListeningTentacleEndpoint](endpoint, "Listening Tentacle")

	assert.EqualError(t, err, "this deployment target is not a Listening Tentacle deployment target")
}

func TestEndpointAs_MissingEndpoint(t *testing.T) {
	assert.NotPanics(t, func() {
		_, err := shared.EndpointAs[*machines.ListeningTentacleEndpoint](nil, "Listening Tentacle")
		assert.Error(t, err)
	})
}
