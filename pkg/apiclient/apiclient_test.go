package apiclient_test

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const PlaceholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

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
	t.Run("GetSpacedClient returns an error when no space is specified", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		rt.EnqueueResponder("GET", "/api/spaces", func(r *http.Request) (any, error) {
			return spaces.Spaces{Items: []*spaces.Space{
				spaces.NewSpace("Integrations"),
			}}, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, "Cannot use specified space ''. Error: cannot find the item", err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient returns an error when a space with the wrong name is specified", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		// first it guesses that we might have a space ID
		rt.EnqueueRawResponder("GET", "/api/spaces/Integrations", func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 404}, nil
		})

		// then it tries a partial name search
		rt.EnqueueResponder("GET", "/api/spaces?partialName=Integrations", func(r *http.Request) (any, error) {
			return spaces.Spaces{Items: []*spaces.Space{
				spaces.NewSpace("NotIntegrations"),
			}}, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "Integrations", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, "Cannot use specified space 'Integrations'. Error: cannot find the item", err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space ID is directly specified", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()

		space7responder := func(r *http.Request) (any, error) {
			space7 := spaces.NewSpace("Integrations")
			space7.ID = "Spaces-7"
			return space7, nil
		}

		testutil.EnqueueRootResponder(rt)

		rt.EnqueueResponder("GET", "/api/spaces/Spaces-7", space7responder)

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(rt)

		// note it just goes for /api/Spaces-7 this time
		rt.EnqueueResponder("GET", "/api/Spaces-7", space7responder)

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "Spaces-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient works when the Space Name is directly specified", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		integrationsSpace := spaces.NewSpace("Integrations")
		integrationsSpace.ID = "Spaces-7"

		rt.EnqueueRawResponder("GET", "/api/spaces/Integrations", func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 404}, nil
		})

		rt.EnqueueResponder("GET", "/api/spaces?partialName=Integrations", func(r *http.Request) (any, error) {
			return spaces.Spaces{Items: []*spaces.Space{integrationsSpace}}, nil
		})

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(rt)

		// note it just goes for /api/Spaces-7 this time
		rt.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) {
			return integrationsSpace, nil
		})

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "Integrations", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient called twice returns the same client instance without additional requests", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()

		integrationsSpace := spaces.NewSpace("Integrations")
		integrationsSpace.ID = "Spaces-7"

		testutil.EnqueueRootResponder(rt)

		rt.EnqueueResponder("GET", "/api/spaces/Spaces-7", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		// we need to enqueue this again because after it finds Spaces-7 it will recreate the client and reload the root.
		testutil.EnqueueRootResponder(rt)

		// note it just goes for /api/Spaces-7 this time
		rt.EnqueueResponder("GET", "/api/Spaces-7", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "Spaces-7", nil)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, rt.RemainingQueueLength())

		// we haven't queued any responders, so if this makes any API requests the test will fail
		apiClient2, err := factory2.GetSpacedClient()
		assert.Same(t, apiClient, apiClient2)
	})
}

func TestClient_GetSpacedClient_Prompt(t *testing.T) {
	t.Run("GetSpacedClient auto-selects the first space when only one exists", func(t *testing.T) {
		integrationsSpace := spaces.NewSpace("Integrations")
		integrationsSpace.ID = "Spaces-23"

		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		rt.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
			// we just have one space here; note spaces.all just returns an array there's no outer wrapper object
			return []*spaces.Space{integrationsSpace}, nil
		})

		// after it gets all the spaces it will restart
		testutil.EnqueueRootResponder(rt)

		// note it just goes for /api/Spaces-7 this time
		rt.EnqueueResponder("GET", "/api/Spaces-23", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		// question/answer doesn't matter, just the presence of the mock signals it's allowed to auto-select the space
		asker, unasked := testutil.NewAskMocker(t, []testutil.QA{})
		defer unasked()

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient prompts for selection when more than one space exists", func(t *testing.T) {
		integrationsSpace := spaces.NewSpace("Integrations")
		integrationsSpace.ID = "Spaces-23"

		cloudSpace := spaces.NewSpace("Cloud")
		cloudSpace.ID = "Spaces-39"

		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		rt.EnqueueResponder("GET", "/api/spaces/all", func(r *http.Request) (any, error) {
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

		// after it gets all the spaces it will restart
		testutil.EnqueueRootResponder(rt)

		// note it just goes for the Space this time
		rt.EnqueueResponder("GET", "/api/Spaces-39", func(r *http.Request) (any, error) { return integrationsSpace, nil })

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})

	t.Run("GetSpacedClient returns an error when a space with the wrong name is specified", func(t *testing.T) {
		rt := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(rt)

		// first it guesses that we might have a space ID
		rt.EnqueueRawResponder("GET", "/api/spaces/Integrations", func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 404}, nil
		})

		// then it tries a partial name search
		rt.EnqueueResponder("GET", "/api/spaces?partialName=Integrations", func(r *http.Request) (any, error) {
			return spaces.Spaces{Items: []*spaces.Space{
				spaces.NewSpace("NotIntegrations"),
			}}, nil
		})

		// question/answer doesn't matter, just the presence of the mock signals it's allowed to auto-select the space
		asker, unasked := testutil.NewAskMocker(t, []testutil.QA{})
		defer unasked()

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(rt), "http://server", PlaceholderApiKey, "Integrations", asker)

		apiClient, err := factory2.GetSpacedClient()
		assert.Nil(t, apiClient)
		assert.Equal(t, "Cannot use specified space 'Integrations'. Error: cannot find the item", err.Error()) // some strongly-typed errors would probably be nicer
		assert.Equal(t, 0, rt.RemainingQueueLength())
	})
}
