package apiclient_test

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"testing"
)

const serverUrl = "http://server"
const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var root = testutil.NewRootResource()

func TestClient_GetSystemClient(t *testing.T) {
	api := testutil.NewMockHttpServer()

	t.Run("GetSystemClient returns the client", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "", nil)
		clientReceiver := testutil.GoBegin2(factory.GetSystemClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		systemClient, err := testutil.ReceivePair(clientReceiver)
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		assert.NotNil(t, systemClient)
	})

	t.Run("GetSystemClient called twice returns the same client instance", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "", nil)
		clientReceiver := testutil.GoBegin2(factory.GetSystemClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		systemClient, err := testutil.ReceivePair(clientReceiver)
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		// Note that if this were to invoke any network requests, the test would fail because the mock API is not
		// prepared for that
		systemClient2, err := factory.GetSystemClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}
		assert.Same(t, systemClient, systemClient2)
	})
}

func TestClient_GetSpacedClient_NoPrompt_NewStyle(t *testing.T) {
	integrationsSpace := spaces.NewSpace("Integrations")
	integrationsSpace.ID = "Spaces-7"

	cloudSpace := spaces.NewSpace("Cloud")
	cloudSpace.ID = "Spaces-39"

	spaceNotSpecifiedMessage := "space must be specified when not running interactively; please set the OCTOPUS_SPACE environment variable or specify --space on the command line"

	api := testutil.NewMockHttpServer()

	t.Run("GetSpacedClient returns an error when no space is specified", func(t *testing.T) {
		// this would pass in interactive mode; we'd auto select the space, however we don't want to do
		// that in no-prompt mode because otherwise people could write a CI script that worked due to
		// auto-selection of the first space, which would then unexpectedly break later if someone added a
		// second space to the octopus server
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)
		// it doesn't actually matter how many spaces there are because the CLI doesn't even ask for them

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, apiClient)
		assert.Equal(t, spaceNotSpecifiedMessage, err.Error())
	})

	t.Run("GetSpacedClient returns an error when a space with the wrong name is specified", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "Integrations", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{cloudSpace})

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, apiClient)
		assert.Equal(t, "cannot find space 'Integrations'", err.Error()) // some strongly-typed errors would probably be nicer
	})

	t.Run("GetSpacedClient works when the Space ID is directly specified", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "Spaces-7", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{integrationsSpace})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		// note it just goes for /api/Spaces-7 this time
		api.ExpectRequest(t, "GET", "/api/Spaces-7").RespondWith(integrationsSpace)

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
	})

	t.Run("GetSpacedClient works when the Space ID is directly specified (case insensitive)", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "spaCeS-7", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{integrationsSpace})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		// note it just goes for /api/Spaces-7 this time
		api.ExpectRequest(t, "GET", "/api/Spaces-7").RespondWith(integrationsSpace)

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
	})

	t.Run("GetSpacedClient works when the Space Name is directly specified", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "Integrations", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{integrationsSpace})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		// note it just goes for /api/Spaces-7 this time
		api.ExpectRequest(t, "GET", "/api/Spaces-7").RespondWith(integrationsSpace)

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
	})

	t.Run("GetSpacedClient works when the Space Name is directly specified (case insensitive)", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "iNtegrationS", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{integrationsSpace})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		// note it just goes for /api/Spaces-7 this time
		api.ExpectRequest(t, "GET", "/api/Spaces-7").RespondWith(integrationsSpace)

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
	})

	t.Run("GetSpacedClient will select by name in preference to ID where there is a collision", func(t *testing.T) {
		missedSpace := spaces.NewSpace("Missed")
		missedSpace.ID = "Spaces-7"

		spaces7space := spaces.NewSpace("Spaces-7") // nobody would do this in reality, but our software must still work properly
		spaces7space.ID = "Spaces-209"

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "Spaces-7", nil)

		clientReceiver := testutil.GoBegin2(factory2.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{
			missedSpace,
			spaces7space,
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/Spaces-209").RespondWith(spaces7space)

		apiClient, err := testutil.ReceivePair(clientReceiver)

		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
	})

	t.Run("GetSpacedClient called twice returns the same client instance without additional requests", func(t *testing.T) {
		factory, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "Integrations", nil)

		clientReceiver := testutil.GoBegin2(factory.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{integrationsSpace})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		// note it just goes for /api/Spaces-7 this time
		api.ExpectRequest(t, "GET", "/api/Spaces-7").RespondWith(integrationsSpace)

		apiClient, err := testutil.ReceivePair(clientReceiver)
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)

		// this isn't in a goroutine so the test will crash if it were to make any network calls
		apiClient2, err := factory.GetSpacedClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}
		assert.Same(t, apiClient, apiClient2)
	})
}
