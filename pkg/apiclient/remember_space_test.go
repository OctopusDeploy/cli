package apiclient_test

import (
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSpace(id, name string) *spaces.Space {
	space := spaces.NewSpace(name)
	space.ID = id
	return space
}

func rememberingFactory(t *testing.T, api *testutil.MockHttpServer, asker *testutil.AskMocker, saved *string) apiclient.ClientFactory {
	t.Helper()

	credential, err := octopusApiClient.NewApiKey(apiKey)
	require.NoError(t, err)

	factory, err := apiclient.NewClientFactory(
		testutil.NewMockHttpClientWithTransport(api), hostUrl, credential, "",
		question.NewAskProvider(asker.AsAsker()))
	require.NoError(t, err)

	factory.(*apiclient.Client).RememberSpace = func(space string) {
		*saved = space
	}
	return factory
}

func twoSpaces() []*spaces.Space {
	return []*spaces.Space{newSpace("Spaces-1", "Default"), newSpace("Spaces-2", "Research")}
}

// respondToSdkInit handles the two requests the SDK makes when it builds the
// system client, before any space lookup happens.
func respondToSdkInit(t *testing.T, api *testutil.MockHttpServer) {
	t.Helper()
	api.ExpectRequest(t, "GET", "/api/").RespondWith(testutil.NewRootResource())
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(map[string]any{
		"Items": []any{}, "ItemsPerPage": 30, "TotalResults": 0,
	})
}

// Being asked which space to use on every command is tedious, so a space the
// person actually picked becomes the default.
func TestGetSpacedClient_RemembersAnExplicitlyChosenSpace(t *testing.T) {
	api, qa := testutil.NewMockServerAndAsker()

	var saved string
	factory := rememberingFactory(t, api, qa, &saved)

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		_, err := factory.GetSpacedClient(&apiclient.FakeRequesterContext{})
		return err
	})

	respondToSdkInit(t, api)
	api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith(twoSpaces())

	_ = qa.ExpectQuestion(t, &survey.Select{
		Message: "You have not specified a Space. Please select one:",
		Options: []string{"Default", "Research"},
	}).AnswerWith("Research")

	// Building the space-scoped client repeats the SDK handshake.
	api.ExpectRequest(t, "GET", "/api/").RespondWith(testutil.NewRootResource())
	api.ExpectRequest(t, "GET", "/api/Spaces-2").RespondWith(testutil.NewRootResource())

	testutil.AssertSuccess(t, <-errReceiver)

	assert.Equal(t, "Research", saved, "the chosen space should become the default")
}

// A server with one space asks nothing, so there is no choice to remember.
func TestGetSpacedClient_DoesNotRememberASpaceThatWasNeverChosen(t *testing.T) {
	api, qa := testutil.NewMockServerAndAsker()

	var saved string
	factory := rememberingFactory(t, api, qa, &saved)

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		_, err := factory.GetSpacedClient(&apiclient.FakeRequesterContext{})
		return err
	})

	respondToSdkInit(t, api)
	api.ExpectRequest(t, "GET", "/api/spaces/all").
		RespondWith([]*spaces.Space{newSpace("Spaces-1", "Default")})
	api.ExpectRequest(t, "GET", "/api/").RespondWith(testutil.NewRootResource())
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(testutil.NewRootResource())

	testutil.AssertSuccess(t, <-errReceiver)

	assert.Empty(t, saved)
}
