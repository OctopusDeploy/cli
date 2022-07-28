package factory

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/briandowns/spinner"
)

type factory struct {
	client  apiclient.ClientFactory
	asker   question.Asker
	spinner *spinner.Spinner
}

type Factory interface {
	GetSystemClient() (*client.Client, error)
	GetSpacedClient() (*client.Client, error)
	Spinner() *spinner.Spinner
	Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

func New(clientFactory apiclient.ClientFactory, asker question.Asker, s *spinner.Spinner) Factory {
	return &factory{
		client:  clientFactory,
		asker:   asker,
		spinner: s,
	}
}

func (f *factory) GetSystemClient() (*client.Client, error) {
	f.spinner.Start()
	defer f.spinner.Stop()
	return f.client.GetSystemClient()
}

func (f *factory) GetSpacedClient() (*client.Client, error) {
	f.spinner.Start()
	defer f.spinner.Stop()
	return f.client.GetSpacedClient()
}

func (f *factory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return f.asker(p, response, opts...)
}

func (f *factory) Spinner() *spinner.Spinner {
	return f.spinner
}
