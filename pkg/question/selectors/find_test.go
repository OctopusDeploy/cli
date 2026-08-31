package selectors_test

import (
	"net/url"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var findRootResource = testutil.NewRootResource()

const findSpaceID = "Spaces-1"
const findProjectID = "Projects-22"

// beginRequest spins up a mock server and hands back the client to run `action` against;
// the octopus client makes network calls on construction so it has to live in the goroutine
func beginRequest[T any](api *testutil.MockHttpServer, action func(octopus *octopusApiClient.Client) (T, error)) chan testutil.Pair[T, error] {
	return testutil.GoBegin2(func() (T, error) {
		defer api.Close()
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		return action(octopus)
	})
}

func TestFindEnvironments(t *testing.T) {
	devEnvironment := fixtures.NewEnvironment(findSpaceID, "Environments-12", "dev")
	prodEnvironment := fixtures.NewEnvironment(findSpaceID, "Environments-13", "production")

	// an environment which is *named* like an ID, to prove the precedence rule
	decoyEnvironment := fixtures.NewEnvironment(findSpaceID, "Environments-99", "Environments-13")

	allEnvironments := []*environments.Environment{devEnvironment, prodEnvironment, decoyEnvironment}

	tests := []struct {
		name        string
		identifiers []string
		expectedIDs []string
		expectedErr string
	}{
		{"finds an environment by name", []string{"dev"}, []string{devEnvironment.ID}, ""},
		{"finds an environment by name, ignoring case", []string{"DEV"}, []string{devEnvironment.ID}, ""},
		{"finds an environment by ID", []string{"Environments-12"}, []string{devEnvironment.ID}, ""},
		{"finds several environments at once", []string{"Environments-12", "production"}, []string{devEnvironment.ID, prodEnvironment.ID}, ""},
		{"prefers an ID match over a name match", []string{"Environments-13"}, []string{prodEnvironment.ID}, ""},
		{"errors when nothing matches", []string{"Environments-404"}, nil, "cannot find an environment with the ID or name of 'Environments-404'"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := testutil.NewMockHttpServer()
			receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]*environments.Environment, error) {
				return selectors.FindEnvironments(octopus, test.identifiers)
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
			api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith(allEnvironments)

			result, err := testutil.ReceivePair(receiver)
			if test.expectedErr == "" {
				assert.Nil(t, err)
				assert.Equal(t, test.expectedIDs, util.SliceTransform(result, func(env *environments.Environment) string { return env.ID }))
			} else {
				assert.EqualError(t, err, test.expectedErr)
			}
		})
	}
}

func TestResolveEnvironmentNames(t *testing.T) {
	findSpace := fixtures.NewSpace(findSpaceID, "Default")
	devEnvironment := fixtures.NewEnvironment(findSpaceID, "Environments-12", "dev")
	ephemeralEnvironment := fixtures.NewEphemeralEnvironment(findSpaceID, "Environments-123", "Ephemeral Environment", "Environments-12")

	t.Run("resolves a mix of regular and ephemeral environments", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]string, error) {
			return selectors.ResolveEnvironmentNames(octopus, findSpace, []string{"DEV", "Environments-123"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{devEnvironment})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/v2?skip=0&take=2147483647&type=Ephemeral").RespondWith(resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items: []*ephemeralenvironments.EphemeralEnvironment{ephemeralEnvironment},
		})

		result, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []string{"dev", "Ephemeral Environment"}, result)
	})

	t.Run("doesn't look at ephemeral environments when everything resolves", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]string, error) {
			return selectors.ResolveEnvironmentNames(octopus, findSpace, []string{"Environments-12"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{devEnvironment})

		result, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []string{"dev"}, result)
	})

	t.Run("names the environment that is actually missing", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]string, error) {
			return selectors.ResolveEnvironmentNames(octopus, findSpace, []string{"dev", "Environments-404"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{devEnvironment})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/v2?skip=0&take=2147483647&type=Ephemeral").RespondWith(resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
			Items: []*ephemeralenvironments.EphemeralEnvironment{ephemeralEnvironment},
		})

		_, err := testutil.ReceivePair(receiver)
		assert.EqualError(t, err, "cannot find an environment with the ID or name of 'Environments-404'")
	})
}

func TestFindEnvironment(t *testing.T) {
	devEnvironment := fixtures.NewEnvironment(findSpaceID, "Environments-12", "dev")

	api := testutil.NewMockHttpServer()
	receiver := beginRequest(api, func(octopus *octopusApiClient.Client) (*environments.Environment, error) {
		return selectors.FindEnvironment(octopus, "Environments-12")
	})

	api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{devEnvironment})

	result, err := testutil.ReceivePair(receiver)
	assert.Nil(t, err)
	assert.Equal(t, devEnvironment.ID, result.ID)
}

func TestFindChannel(t *testing.T) {
	project := fixtures.NewProject(findSpaceID, findProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+findProjectID)

	defaultChannel := fixtures.NewChannel(findSpaceID, "Channels-1", "Default", findProjectID)
	betaChannel := fixtures.NewChannel(findSpaceID, "Channels-2", "Beta", findProjectID)

	// a channel which is *named* like an ID, to prove the precedence rule
	decoyChannel := fixtures.NewChannel(findSpaceID, "Channels-3", "Channels-2", findProjectID)

	allChannels := []*channels.Channel{defaultChannel, betaChannel, decoyChannel}

	tests := []struct {
		name        string
		identifier  string
		expectedID  string
		expectedErr string
	}{
		{"finds a channel by name", "Beta", betaChannel.ID, ""},
		{"finds a channel by name, ignoring case", "beta", betaChannel.ID, ""},
		{"finds a channel by ID", "Channels-1", defaultChannel.ID, ""},
		{"prefers an ID match over a name match", "Channels-2", betaChannel.ID, ""},
		{"errors when nothing matches", "Channels-404", "", "cannot find a channel in project 'Fire Project' with the ID or name of 'Channels-404'"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := testutil.NewMockHttpServer()
			receiver := beginRequest(api, func(octopus *octopusApiClient.Client) (*channels.Channel, error) {
				return selectors.FindChannel(octopus, project, test.identifier)
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
			api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+findProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: allChannels,
			})

			result, err := testutil.ReceivePair(receiver)
			if test.expectedErr == "" {
				assert.Nil(t, err)
				assert.Equal(t, test.expectedID, result.ID)
			} else {
				assert.EqualError(t, err, test.expectedErr)
			}
		})
	}
}

func TestFindTenants(t *testing.T) {
	cokeTenant := fixtures.NewTenant(findSpaceID, "Tenants-29", "Coke", "Regions/us-east")

	t.Run("finds a tenant by ID", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]*tenants.Tenant, error) {
			return selectors.FindTenants(octopus, []string{"Tenants-29"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/Tenants-29").RespondWith(cokeTenant)

		result, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []string{cokeTenant.ID}, util.SliceTransform(result, func(tenant *tenants.Tenant) string { return tenant.ID }))
	})

	t.Run("falls back to a name lookup when the ID doesn't exist", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]*tenants.Tenant, error) {
			return selectors.FindTenants(octopus, []string{"Coke"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/Coke").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?partialName=Coke").RespondWith(resources.Resources[*tenants.Tenant]{
			Items: []*tenants.Tenant{cokeTenant},
		})

		result, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []string{cokeTenant.ID}, util.SliceTransform(result, func(tenant *tenants.Tenant) string { return tenant.ID }))
	})

	t.Run("finds an exact name match beyond the first page of the partial name search", func(t *testing.T) {
		// `partialName` is a contains filter, so a tenant exactly named "Smith" can be pushed off
		// the first page by every other tenant whose name also contains "Smith"
		aaronSmith := fixtures.NewTenant(findSpaceID, "Tenants-30", "Aaron Smith")
		smith := fixtures.NewTenant(findSpaceID, "Tenants-31", "Smith")

		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]*tenants.Tenant, error) {
			return selectors.FindTenants(octopus, []string{"Smith"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/Smith").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?partialName=Smith").RespondWith(resources.Resources[*tenants.Tenant]{
			Items: []*tenants.Tenant{aaronSmith},
			PagedResults: resources.PagedResults{
				Links: resources.Links{PageNext: "/api/Spaces-1/tenants?partialName=Smith&skip=1"},
			},
		})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?partialName=Smith&skip=1").RespondWith(resources.Resources[*tenants.Tenant]{
			Items: []*tenants.Tenant{smith},
		})

		result, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []string{smith.ID}, util.SliceTransform(result, func(tenant *tenants.Tenant) string { return tenant.ID }))
	})

	t.Run("errors when nothing matches", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		receiver := beginRequest(api, func(octopus *octopusApiClient.Client) ([]*tenants.Tenant, error) {
			return selectors.FindTenants(octopus, []string{"Tenants-404"})
		})

		api.ExpectRequest(t, "GET", "/api/").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(findRootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants/Tenants-404").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?partialName=Tenants-404").RespondWith(resources.Resources[*tenants.Tenant]{})

		_, err := testutil.ReceivePair(receiver)
		assert.EqualError(t, err, "cannot find a tenant with the ID or name of 'Tenants-404'")
	})
}
