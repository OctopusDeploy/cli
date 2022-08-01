package apiclient_test

import (
	"net/http"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
)

const serverUrl = "http://server"
const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func TestClient_GetSystemClient(t *testing.T) {
	httpClient := testutil.NewMockHttpClient()
	factory, _ := apiclient.NewClientFactory(httpClient, "http://some-host", "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "", nil)

	t.Run("GetSystemClient returns the client", func(t *testing.T) {
		systemClient, err := factory.GetSystemClient()
		if !testutil.EnsureSuccess(t, err) {
			return
		}

		assert.NotNil(t, systemClient)
	})

	t.Run("GetSystemClient called twice returns the same client instance", func(t *testing.T) {
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

func TestClient_GetSpacedClient_NoPrompt(t *testing.T) {
	integrationsSpace := spaces.NewSpace("Integrations")
	integrationsSpace.ID = "Spaces-7"

	cloudSpace := spaces.NewSpace("Cloud")
	cloudSpace.ID = "Spaces-39"

	spaceNotSpecifiedMessage := "space must be specified when not running interactively; please set the OCTOPUS_SPACE environment variable or specify --space on the command line"

	t.Run("GetSpacedClient returns an error when no space is specified and only one space exists", func(t *testing.T) {
		// this would pass in interactive mode; we'd auto select the space, however we don't want to do
		// that in no-prompt mode because otherwise people could write a CI script that worked due to
		// auto-selection of the first space, which would then unexpectedly break later if someone added a
		// second space to the octopus server
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api) // even though the config is invalid it still hits /api to check auth, etc

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, spaceNotSpecifiedMessage, err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient returns an error when no space is specified and more than one space exists", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api) // even though the config is invalid it still hits /api to check auth, etc

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, spaceNotSpecifiedMessage, err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient returns an error when a space with the wrong name is specified", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{cloudSpace}, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Integrations", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, "cannot find space 'Integrations'", err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space ID is directly specified", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()

		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace}, nil
		})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(api)

		// note it just goes for /api/Spaces-7 this time
		api.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Spaces-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space ID is directly specified (case insensitive)", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()

		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace}, nil
		})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(api)

		// note it just goes for /api/Spaces-7 this time
		api.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "spaCeS-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space Name is directly specified", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace}, nil
		})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(api)

		// note it just goes for /api/Spaces-7 this time
		api.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Integrations", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space Name is directly specified (case insensitive)", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace}, nil
		})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(api)

		// note it just goes for /api/Spaces-7 this time
		api.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "iNtegrationS", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient will select by name in preference to ID where there is a collision", func(t *testing.T) {
		missedSpace := spaces.NewSpace("Missed")
		missedSpace.ID = "Spaces-7"

		spaces7space := spaces.NewSpace("Spaces-7") // nobody would do this in reality, but our software must still work properly
		spaces7space.ID = "Spaces-209"

		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{
				missedSpace,
				spaces7space,
			}, nil
		})

		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/Spaces-209", func(r *http.Request) (any, error) {
			return spaces7space, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Spaces-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient called twice returns the same client instance without additional requests", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace}, nil
		})

		testutil.EnqueueRootResponder(api)
		api.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Spaces-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())

		// we haven't queued any responders, so if this makes any API requests the test will fail
		apiClient2, _ := factory2.GetSpacedClient()
		assert.Same(t, apiClient, apiClient2)
	})
}

func TestClient_GetSpacedClient_Prompt(t *testing.T) {
	integrationsSpace := spaces.NewSpace("Integrations")
	integrationsSpace.ID = "Spaces-23"

	t.Run("GetSpacedClient auto-selects the first space when only one exists", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			// we just have one space here; note spaces.all just returns an array there's no outer wrapper object
			return []*spaces.Space{integrationsSpace}, nil
		})

		// after it gets all the spaces it will restart and go directly for the specific space to get the links
		testutil.EnqueueRootResponder(api)
		api.EnqueueResponder("GET", "/api/Spaces-23", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		// question/answer doesn't matter, just the presence of the mock signals it's allowed to auto-select the space
		asker, unasked := testutil.NewAskMocker(t, []testutil.QA{})
		defer unasked()

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient prompts for selection when more than one space exists", func(t *testing.T) {
		cloudSpace := spaces.NewSpace("Cloud")
		cloudSpace.ID = "Spaces-39"

		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{integrationsSpace, cloudSpace}, nil
		})

		// make sure it asks us to select a space, and respond with "Cloud"
		asker, unasked := testutil.NewAskOneMocker(t, testutil.QA{
			Prompt: &survey.Select{
				Message: "You have not specified a Space. Please select one:",
				Options: []string{"Integrations", "Cloud"},
			},
			Answer: "Cloud",
		})
		defer unasked()

		// after it gets all the spaces it will restart and go directly for the specific space to get the links
		testutil.EnqueueRootResponder(api)
		api.EnqueueResponder("GET", "/api/Spaces-39", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("GetSpacedClient returns an error when a space with the wrong name is specified", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)

		cloudSpace := spaces.NewSpace("CloudSpace")
		cloudSpace.ID = "Spaces-39"

		// then it tries a partial name search
		api.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			return []*spaces.Space{cloudSpace}, nil
		})

		// question/answer doesn't matter, just the presence of the mock signals it's allowed to auto-select the space
		asker, unasked := testutil.NewAskMocker(t, []testutil.QA{})
		defer unasked()

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Integrations", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, "cannot find space 'Integrations'", err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, api.RemainingQueueLength())
	})
}
