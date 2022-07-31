package factory

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
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
	GetCurrentSpace() *spaces.Space
	IsPromptEnabled() bool
	SetPromptDisabled()
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

func (f *factory) GetCurrentSpace() *spaces.Space {
	return f.client.GetActiveSpace()
}

func (f *factory) IsPromptEnabled() bool {
	return f.asker != nil
}

// SetPromptDisabled prevents the CLI from prompting for user input.
// Note this is a one-way function; once we disable it, we can't re-enable
func (f *factory) SetPromptDisabled() {
	f.asker = nil
}

func (f *factory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	if f.asker == nil {
		// this shouldn't happen; commands should check IsPromptEnabled before attempting to prompt
		return &cliErrors.PromptDisabledError{}
	}
	return f.asker(p, response, opts...)
}

func (f *factory) Spinner() *spinner.Spinner {
	return f.spinner
}
