package apiclient_test

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient_Get_Clients(t *testing.T) {
	httpClient := testutil.NewMockHttpClient()

	factory, err := apiclient.NewClientFactory(httpClient, "http://some-host", "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "")
	if !testutil.EnsureSuccess(t, err) {
		return
	}

	t.Run("first call returns the client", func(t *testing.T) {
		systemClient, err := factory.GetSystemClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		assert.NotNil(t, systemClient)
	})

	t.Run("calling twice returns the same client instance", func(t *testing.T) {
		systemClient, err := factory.GetSystemClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		systemClient2, err := factory.GetSystemClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		assert.Same(t, systemClient, systemClient2)
	})

}
