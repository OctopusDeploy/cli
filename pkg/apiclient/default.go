package apiclient

import (
	"errors"
	"fmt"
	octopusErrors "github.com/OctopusDeploy/cli/pkg/errors"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/hashicorp/go-multierror"
	"net/http"
	"net/url"
	"os"
)

type ClientFactory interface {
	// GetSpacedClient returns an Octopus api Client instance which is bound to the Space
	// specified in the OCTOPUS_SPACE environment variable, or the command line. It should be the default
	GetSpacedClient() (*octopusApiClient.Client, error)

	// GetSystemClient returns an Octopus api Client instance which isn't bound to any Space.
	// Use it for things that live outside of a space, such as Teams, or Spaces themselves
	GetSystemClient() (*octopusApiClient.Client, error)
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

	// the Server URL, obtained from OCTOPUS_HOST
	ApiUrl *url.URL
	// the Octopus API Key, obtained from OCTOPUS_API_KEY
	ApiKey string
	// the Octopus Space to work within. Obtained from OCTOPUS_SPACE (TODO: or --space=XYZ on the command line??)
	// Required for commands that need a space, but may be omitted for server-wide commands such as listing teams
	Space string
}

func NewClientFactory(httpClient *http.Client, host string, apiKey string, space string) (ClientFactory, error) {
	hostUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	clientImpl := &Client{
		HttpClient:        httpClient,
		SystemClient:      nil,
		SpaceScopedClient: nil,
		ApiUrl:            hostUrl,
		ApiKey:            apiKey,
		Space:             space,
	}
	return clientImpl, nil
}

// NewFromEnvironment Creates a new Client wrapper structure by reading the environment.
// specifies nil for the HTTP Client, so this is not for unit tests; use NewClientFactory(... instead)
func NewClientFactoryFromEnvironment() (ClientFactory, error) {
	host := os.Getenv("OCTOPUS_HOST")
	apiKey := os.Getenv("OCTOPUS_API_KEY")
	space := os.Getenv("OCTOPUS_SPACE")

	errs := ValidateMandatoryEnvironment(host, apiKey)
	if errs != nil {
		return nil, errs
	}

	return NewClientFactory(nil, host, apiKey, space)
}

func ValidateMandatoryEnvironment(host string, apiKey string) error {
	var result *multierror.Error

	if host == "" {
		result = multierror.Append(result, &octopusErrors.OsEnvironmentError{EnvironmentVariable: "OCTOPUS_HOST"})
	}
	if apiKey == "" {
		result = multierror.Append(result, &octopusErrors.OsEnvironmentError{EnvironmentVariable: "OCTOPUS_API_KEY"})
	}

	return result.ErrorOrNil()
}

func (c *Client) GetSpacedClient() (*octopusApiClient.Client, error) {
	if c.SpaceScopedClient != nil {
		return c.SpaceScopedClient, nil
	}

	// logic here is a bit fiddly:
	// We could have been given either a space name, or a space ID, so we need to use the SystemClient to go look it up.
	systemClient, err := c.GetSystemClient()
	if err != nil {
		return nil, err
	}

	// TODO if the caller has not specified a space, prompt interactively

	/* TODO: There was some discussion around having this code just pick the first space (if there is only one) in
	situations where the caller has not supplied a space. Do we want to still do that? In which case we need to GetAll on the spaces, not just GetByIdOrName */

	// TODO: Are we supposed to match a space by name first or by ID first? ID seems more reasonable, but confirm that
	space, err := systemClient.Spaces.GetByIDOrName(c.Space)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot use specified space '%s'. Error: %s", c.Space, err))
	}
	// ok we found a space

	scopedClient, err := octopusApiClient.NewClient(c.HttpClient, c.ApiUrl, c.ApiKey, space.GetID())
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SpaceScopedClient = scopedClient
	return scopedClient, nil
}

func (c *Client) GetSystemClient() (*octopusApiClient.Client, error) {
	// they are specifically asking for the System Client, return or create it if need be
	if c.SystemClient != nil {
		return c.SystemClient, nil
	}

	systemClient, err := octopusApiClient.NewClient(c.HttpClient, c.ApiUrl, c.ApiKey, "") // deliberate empty string for space here
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SystemClient = systemClient
	return systemClient, nil
}
