package apiclient

import (
	"errors"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"net/url"
	"os"
)

type ClientFactory interface {
	Get(spaceScoped bool) (*octopusApiClient.Client, error)
}

type Client struct {
	// nullable, lazily created by Get()
	// TODO this should be an interface rather than a struct, but this requires changing the SDK, we'll get round to that
	Client *octopusApiClient.Client
	// the Server URL, obtained from OCTOPUS_HOST
	ApiUrl *url.URL
	// the Octopus API Key, obtained from OCTOPUS_API_KEY
	ApiKey string
	// the Octopus Space to work within. Obtained from OCTOPUS_SPACE (TODO: or --space=XYZ on the command line??)
	// Required for commands that need a space, but may be omitted for server-wide commands such as listing teams
	Space string
}

// Creates a new Client wrapper structure
func NewFromEnvironment() (ClientFactory, error) {
	host := os.Getenv("OCTOPUS_HOST")
	apiKey := os.Getenv("OCTOPUS_API_KEY")
	space := os.Getenv("OCTOPUS_SPACE")

	if host == "" {
		// TODO a proper set of Error types
		return nil, errors.New("OCTOPUS_HOST environment variable is missing or blank")
	}

	hostUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	clientImpl := &Client{
		Client: nil,
		ApiUrl: hostUrl,
		ApiKey: apiKey,
		Space:  space,
	}
	return clientImpl, nil
}

func (c *Client) Get(spaceScoped bool) (*octopusApiClient.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}

	// TODO space selection logic. c.Space could be an ID or a name, we might need to look it up
	// or maybe even prompt the user.
	octopusClient, err := octopusApiClient.NewClient(nil, c.ApiUrl, c.ApiKey, c.Space)
	if err != nil {
		return nil, err
	}
	// stash for future use
	c.Client = octopusClient
	return octopusClient, nil
}
