package apiclient

import (
	"net/url"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

type Client struct {
	Client *client.Client
	ApiUrl *url.URL
	ApiKey string
	Space  string
}

func New() *Client {
	return &Client{
		Client: nil,
		ApiUrl: nil,
		ApiKey: "",
		Space:  "",
	}
}

func (c *Client) Get() {

}
