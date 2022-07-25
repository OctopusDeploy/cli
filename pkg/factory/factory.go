package factory

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

type factory struct {
	client apiclient.ClientFactory
	asker  question.Asker
}

type Factory interface {
	Client(spaceScoped bool) (*client.Client, error)
	Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

func New(clientFactory apiclient.ClientFactory, asker question.Asker) Factory {
	return &factory{
		client: clientFactory,
		asker:  asker,
	}
}

func (f *factory) Client(spaceScoped bool) (*client.Client, error) {
	return f.client.Get(spaceScoped)
}

func (f *factory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return f.asker(p, response, opts...)
}
