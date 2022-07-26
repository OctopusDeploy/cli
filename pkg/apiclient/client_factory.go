package apiclient

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"net/url"
	"os"

	octopusErrors "github.com/OctopusDeploy/cli/pkg/errors"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/hashicorp/go-multierror"
	"net/http"
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

	// Handle out to prompt the user for things. If this is nil, it means we're in no-prompt mode
	Ask question.Asker
}

func NewClientFactory(httpClient *http.Client, host string, apiKey string, space string, asker question.Asker) (ClientFactory, error) {
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
		Ask:               asker,
	}
	return clientImpl, nil
}

// NewClientFactoryFromEnvironment Creates a new Client wrapper structure by reading the environment.
// specifies nil for the HTTP Client, so this is not for unit tests; use NewClientFactory(... instead)
func NewClientFactoryFromEnvironment() (ClientFactory, error) {
	host := os.Getenv("OCTOPUS_HOST")
	apiKey := os.Getenv("OCTOPUS_API_KEY")
	space := os.Getenv("OCTOPUS_SPACE")

	var ask question.Asker = nil

	// TODO put this in some other function as we may check many things to determine if we're suppressing prompting
	_, ci := os.LookupEnv("CI")
	if !ci {
		ask = survey.AskOne
	}

	errs := ValidateMandatoryEnvironment(host, apiKey)
	if errs != nil {
		return nil, errs
	}

	return NewClientFactory(nil, host, apiKey, space, ask)
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

	// if the caller has not specified a space, prompt interactively
	var spaceID string
	// if c.Ask is nil it means we're in automation mode.
	if c.Space == "" && c.Ask != nil {
		allSpaces, err := systemClient.Spaces.GetAll()
		if err != nil {
			return nil, err
		}

		switch len(allSpaces) {
		case 0:
			return nil, errors.New("No spaces found")
		case 1:
			// TODO should we log here that we are inferring the first space?
			// should we assert that it is the DEFAULT space? That feels significant.
			selectedSpace := allSpaces[0]
			c.Space = selectedSpace.ID
			spaceID = selectedSpace.ID
		default:
			selectedSpace, err := question.SelectMap(
				c.Ask,
				"You have not specified a Space. Please select one:", allSpaces, func(item *spaces.Space) string { return item.GetName() })

			if err != nil {
				return nil, err
			}
			c.Space = selectedSpace.ID
			spaceID = selectedSpace.ID
		}
	}

	if spaceID == "" {
		// TODO: Are we supposed to match a space by name first or by ID first? ID seems more reasonable, but confirm that
		space, err := systemClient.Spaces.GetByIDOrName(c.Space)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Cannot use specified space '%s'. Error: %s", c.Space, err))
		}
		// ok we found a space
		spaceID = space.ID
	}

	scopedClient, err := octopusApiClient.NewClient(c.HttpClient, c.ApiUrl, c.ApiKey, spaceID)
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SpaceScopedClient = scopedClient
	c.SystemClient = nil // system client has been "upgraded", no need for it anymore
	return scopedClient, nil
}

func (c *Client) GetSystemClient() (*octopusApiClient.Client, error) {
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

	systemClient, err := octopusApiClient.NewClient(c.HttpClient, c.ApiUrl, c.ApiKey, "") // deliberate empty string for space here
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.SystemClient = systemClient
	return systemClient, nil
}
