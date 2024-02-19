package apiclient

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/viper"

	"net/http"

	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

type ClientFactory interface {
	// GetSpacedClient returns an Octopus api Client instance which is bound to the Space
	// specified in the OCTOPUS_SPACE environment variable, or the command line. It should be the default
	GetSpacedClient(requester Requester) (*octopusApiClient.Client, error)

	// GetSystemClient returns an Octopus api Client instance which isn't bound to any Space.
	// Use it for things that live outside of a space, such as Teams, or Spaces themselves
	GetSystemClient(requester Requester) (*octopusApiClient.Client, error)

	// GetActiveSpace returns the currently selected space.
	// Note this is lazily populated when you call GetSpacedClient;
	// if you have not yet done so then it may return nil
	GetActiveSpace() *spaces.Space

	// SetSpaceNameOrId replaces whichever space name or ID was picked up from the environment or selected
	// interactively. This resets the internal cache inside the ClientFactory, meaning that the next time
	// someone calls GetSpacedClient we will have to query the Octopus Server to look up spaceNameOrId,
	// and any calls to GetActiveSpace before that will return nil
	SetSpaceNameOrId(spaceNameOrId string)

	// GetHostUrl returns the current set API URL as a string
	GetHostUrl() string

	// GetHttpClient returns a raw http client which can be used to query Octopus
	GetHttpClient() (*http.Client, error)
}

type Client struct {
	// Underlying HTTP Client (settable for mocking in unit tests).
	// If nil, will use the system default HTTP client to connect to the Octopus Deploy server
	HttpClient *http.Client

	// TODO this should be an interface rather than a struct, but this requires changing the SDK, we'll get round to that
	// Octopus API Client not scoped to any space. nullable, lazily created by Get()
	SystemClient *octopusApiClient.Client

	// TODO this should be an interface rather than a struct, but this requires changing the SDK, we'll get round to that
	// Octopus API Client scoped to the current space. nullable, lazily created by Get()
	SpaceScopedClient *octopusApiClient.Client

	// the Server URL, obtained from OCTOPUS_URL
	ApiUrl *url.URL
	// Credentials, obtained from OCTOPUS_API_KEY or OCTOPUS_ACCESS_TOKEN
	Credentials octopusApiClient.ICredential
	// the Octopus SpaceNameOrID to work within. Obtained from OCTOPUS_SPACE (TODO: or --space=XYZ on the command line??)
	// Required for commands that need a space, but may be omitted for server-wide commands such as listing teams
	SpaceNameOrID string

	// After the space lookup process has occurred, we cache a reference to the SpaceNameOrID object for future use
	// May be nil if we haven't done space lookup yet
	ActiveSpace *spaces.Space

	Ask question.AskProvider
}

func NewClientFactory(httpClient *http.Client, host string, credentials octopusApiClient.ICredential, spaceNameOrID string, ask question.AskProvider) (ClientFactory, error) {
	// httpClient is allowed to be nil; it is passed through to the go-octopusdeploy library which falls back to a default httpClient
	if host == "" {
		return nil, cliErrors.NewArgumentNullOrEmptyError("host")
	}

	if credentials == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("credentials")
	}

	// space is allowed to be blank, we will prompt for a space in interactive mode, or error if not
	if ask == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("ask")
	}

	hostUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	clientImpl := &Client{
		HttpClient:        httpClient,
		SystemClient:      nil,
		SpaceScopedClient: nil,
		ApiUrl:            hostUrl,
		Credentials:       credentials,
		SpaceNameOrID:     spaceNameOrID,
		ActiveSpace:       nil,
		Ask:               ask,
	}
	return clientImpl, nil
}

// NewClientFactoryFromConfig Creates a new Client wrapper structure by reading the viper config.
// specifies nil for the HTTP Client, so this is not for unit tests; use NewClientFactory(... instead)
func NewClientFactoryFromConfig(ask question.AskProvider) (ClientFactory, error) {
	host := viper.GetString(constants.ConfigUrl)
	apiKey := viper.GetString(constants.ConfigApiKey)
	accessToken := viper.GetString(constants.ConfigAccessToken)
	spaceNameOrID := viper.GetString(constants.ConfigSpace)

	errs := ValidateMandatoryEnvironment(host, apiKey, accessToken, ask.IsInteractive())
	if errs != nil {
		return nil, errs
	}

	var httpClient *http.Client
	if ask.IsInteractive() {
		// spinner round-tripper only needed for interactive mode
		httpClient = &http.Client{
			Transport: NewSpinnerRoundTripper(),
		}
	}

	var credentials octopusApiClient.ICredential

	if apiKey != "" {
		apiKeyCredential, err := octopusApiClient.NewApiKey(apiKey)

		if err != nil {
			return nil, err
		}

		credentials = apiKeyCredential
	} else if accessToken != "" {
		accessTokenCredential, err := octopusApiClient.NewAccessToken(accessToken)

		if err != nil {
			return nil, err
		}

		credentials = accessTokenCredential
	}

	return NewClientFactory(httpClient, host, credentials, spaceNameOrID, ask)
}

func ValidateMandatoryEnvironment(host string, apiKey string, accessToken string, isInteractive bool) error {

	if host == "" || (apiKey == "" && accessToken == "") {

		if isInteractive {
			err := GetInteractiveMandatoryEnvironmentErrorMessage()

			return fmt.Errorf(err)
		}

		err := GetNonInteractiveMandatoryEnvironmentErrorMessage()

		return fmt.Errorf(err)
	}

	return nil
}

func GetInteractiveMandatoryEnvironmentErrorMessage() string {

	octopusLogo := ""

	if viper.GetBool(constants.ConfigShowOctopus) {
		octopusLogo = output.Cyanf(`
%s
`, constants.OctopusLogo)
	}

	return heredoc.Docf(`
Work seamlessly with Octopus Deploy from the command line.
%s
To get started with the Octopus CLI, please login to your Octopus Server using:

  %s

Alternatively you can set the following environment variables:

  %s: The URL of your Octopus Server
  %s: An API key to authenticate to the Octopus Server with
			
Happy deployments!`,
		octopusLogo, output.Cyan("octopus login"), output.Cyan(constants.EnvOctopusUrl), output.Cyan(constants.EnvOctopusApiKey))
}

func GetNonInteractiveMandatoryEnvironmentErrorMessage() string {
	oidcHeader := output.Bold("OpenID Connect (OIDC)")
	apiKeyHeader := output.Bold("API Key")

	oidcLoginCommand := output.Cyan("octopus login --server {OctopusServerUrl} --service-account-id {ServiceAccountId} --id-token {IdTokenFromOidcProvider}")
	apiKeyLoginCommand := output.Cyan("octopus login --server {OctopusServerUrl} --api-key {OctopusApiKey}")

	serverEnvVar := output.Cyan(constants.EnvOctopusUrl)
	accessTokenEnvVar := output.Cyan(constants.EnvOctopusAccessToken)
	apiKeyEnvVar := output.Cyan(constants.EnvOctopusApiKey)

	octopusLogo := ""

	if viper.GetBool(constants.ConfigShowOctopus) {
		octopusLogo = output.Cyanf(`
%s
`, constants.OctopusLogo)
	}

	return heredoc.Docf(`
Work seamlessly with Octopus Deploy from the command line.
%s
The Octopus CLI supports two methods of authentication when using automation:

%s

To use an Octopus access token obtained via OIDC to configure the CLI, set the following environment variables:

  %s: The URL of your Octopus Server
  %s: The access token obtained from the Octopus Server

To exchange an OIDC ID token for an Octopus access token and configure the CLI:

  %s

%s

To use an existing API key to configure the CLI, set the following environment variables:

  %s: The URL of your Octopus Server
  %s: The API key to authenticate to the Octopus Server with

Or alternatively:
  
  %s
			
Happy deployments!`,
		octopusLogo, oidcHeader, serverEnvVar, accessTokenEnvVar, oidcLoginCommand, apiKeyHeader, serverEnvVar, apiKeyEnvVar, apiKeyLoginCommand)
}

func (c *Client) GetActiveSpace() *spaces.Space {
	return c.ActiveSpace
}

func (c *Client) GetHostUrl() string {
	return c.ApiUrl.String()
}

func (c *Client) GetHttpClient() (*http.Client, error) {
	return c.HttpClient, nil
}

func (c *Client) SetSpaceNameOrId(spaceNameOrId string) {
	// technically don't need to nil out the SystemClient, but it's cleaner that way
	// because a SpaceScopedClient can also be a SystemClient
	c.SystemClient = nil

	// nil out all the space-specific stuff
	c.SpaceScopedClient = nil
	c.ActiveSpace = nil
	c.SpaceNameOrID = spaceNameOrId
}

func (c *Client) GetSpacedClient(requester Requester) (*octopusApiClient.Client, error) {
	if c.SpaceScopedClient != nil {
		return c.SpaceScopedClient, nil
	}

	// logic here is a bit fiddly:
	// We could have been given either a space name, or a space ID, so we need to use the SystemClient to go look it up.
	systemClient, err := c.GetSystemClient(requester)
	if err != nil {
		return nil, err
	}

	// if the caller has not specified a space, prompt interactively
	var foundSpaceID string
	// if c.Ask is nil it means we're in automation mode.
	if c.SpaceNameOrID == "" {
		if !c.Ask.IsInteractive() {
			return nil, errors.New("space must be specified when not running interactively; please set the OCTOPUS_SPACE environment variable or specify --space on the command line")
		}

		allSpaces, err := systemClient.Spaces.GetAll()
		if err != nil {
			return nil, err
		}

		switch len(allSpaces) {
		case 0:
			return nil, errors.New("no spaces found")
		case 1:
			selectedSpace := allSpaces[0]
			c.ActiveSpace = selectedSpace
			c.SpaceNameOrID = selectedSpace.ID
			foundSpaceID = selectedSpace.ID
		default:
			selectedSpace, err := question.SelectMap(
				c.Ask.Ask,
				"You have not specified a Space. Please select one:", allSpaces, func(item *spaces.Space) string { return item.GetName() })

			if err != nil {
				return nil, err
			}
			c.ActiveSpace = selectedSpace
			c.SpaceNameOrID = selectedSpace.ID
			foundSpaceID = selectedSpace.ID
		}
	}

	if foundSpaceID == "" {
		// https://github.com/OctopusDeploy/cli/issues/30
		// we prefer to match on Name first, and then fallback to ID; The server doesn't have direct support
		// for that logic so the most pragmatic way to achieve that is to iterate the list of spaces client-side
		allSpaces, err := systemClient.Spaces.GetAll()
		if err != nil {
			return nil, fmt.Errorf("cannot load spaces. Error: %v", err)
		}

		var foundSpace *spaces.Space = nil
		var foundSpaceByID *spaces.Space = nil // second-tier match, only use this if foundSpace is nilt
		for _, space := range allSpaces {
			if strings.EqualFold(space.Name, c.SpaceNameOrID) { // direct hit on the name, this is the one we want
				foundSpace = space
				break
			}
			if strings.EqualFold(space.ID, c.SpaceNameOrID) { // hit on the ID; we prefer name so keep this as a fallback
				foundSpaceByID = space
			}
		}
		if foundSpace == nil && foundSpaceByID != nil {
			foundSpace = foundSpaceByID
		}

		if foundSpace == nil {
			return nil, fmt.Errorf("cannot find space '%s'", c.SpaceNameOrID)
		}
		// ok we found a space
		c.ActiveSpace = foundSpace
		c.SpaceNameOrID = foundSpace.ID
		foundSpaceID = foundSpace.ID
	}

	scopedClient, err := octopusApiClient.NewClientWithCredentials(c.HttpClient, c.ApiUrl, c.Credentials, foundSpaceID, requester.GetRequester())
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SpaceScopedClient = scopedClient
	c.SystemClient = nil // system client has been "upgraded", no need for it anymore
	return scopedClient, nil
}

func (c *Client) GetSystemClient(requester Requester) (*octopusApiClient.Client, error) {
	// Internal quirks of the go-octopusdeploy API SDK:
	// A space-scoped client can do System level things perfectly well, but the inverse is not true.
	// Essentially:
	// - we can always create a "system" client which has a Space ID of empty string
	// - we can only create a "space scoped" client if we have a valid space ID, which requires using the
	//   system client to look up a space ID and test it first.
	// - once we have a "space scoped" client we can use it for all the system things and avoid storing
	//   two client copies in memory, so we can throw out the system client.
	if c.SpaceScopedClient != nil {
		return c.SpaceScopedClient, nil
	}

	if c.SystemClient != nil {
		return c.SystemClient, nil
	}

	systemClient, err := octopusApiClient.NewClientWithCredentials(c.HttpClient, c.ApiUrl, c.Credentials, "", requester.GetRequester()) // deliberate empty string for space here
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SystemClient = systemClient
	return systemClient, nil
}

// NewStubClientFactory returns a stub instance, so you can satisfy external code that needs a ClientFactory
func NewStubClientFactory() ClientFactory {
	return &stubClientFactory{}
}

type stubClientFactory struct{}

func (s *stubClientFactory) GetSpacedClient(_ Requester) (*octopusApiClient.Client, error) {
	return nil, errors.New("app is not configured correctly")
}

func (s *stubClientFactory) GetSystemClient(_ Requester) (*octopusApiClient.Client, error) {
	return nil, errors.New("app is not configured correctly")
}

func (s *stubClientFactory) GetActiveSpace() *spaces.Space { return nil }

func (s *stubClientFactory) SetSpaceNameOrId(_ string) {}

func (s *stubClientFactory) GetHostUrl() string { return "" }

func (s *stubClientFactory) GetHttpClient() (*http.Client, error) {
	return nil, nil
}
