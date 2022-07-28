package factory

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

type factory struct {
	client apiclient.ClientFactory
	asker  question.Asker
}

type Factory interface {
	GetSystemClient() (*client.Client, error)
	GetSpacedClient() (*client.Client, error)
	GetCurrentSpace() *spaces.Space
	Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

func New(clientFactory apiclient.ClientFactory, asker question.Asker) Factory {
	return &factory{
		client: clientFactory,
		asker:  asker,
	}
}

func (f *factory) GetSystemClient() (*client.Client, error) {
	return f.client.GetSystemClient()
}

func (f *factory) GetSpacedClient() (*client.Client, error) {
	return f.client.GetSpacedClient()
}

func (f *factory) GetCurrentSpace() *spaces.Space {
	return f.client.GetCurrentSpace()
}

func (f *factory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return f.asker(p, response, opts...)
}
