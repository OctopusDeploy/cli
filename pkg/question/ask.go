package question

import (
	"github.com/AlecAivazis/survey/v2"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
)

type Asker func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error

// Both the ClientFactory and the main Factory need to reference survey to ask questions,
// but we need a single place to hold the reference to survey so we can switch it off for
// automation mode. This wrapper fills that gap.

type AskProvider interface {
	IsPromptEnabled() bool
	SetPromptDisabled()
	Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

type askWrapper struct {
	asker Asker
}

func NewAskProvider(asker Asker) AskProvider {
	return &askWrapper{
		asker: asker,
	}
}

func (a *askWrapper) IsPromptEnabled() bool {
	return a.asker != nil
}

func (a *askWrapper) SetPromptDisabled() {
	a.asker = nil
}

func (a *askWrapper) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	if a.asker != nil {
		return a.asker(p, response, opts...)
	} else {
		// this shouldn't happen; commands should check IsPromptEnabled before attempting to prompt
		return &cliErrors.PromptDisabledError{}
	}
}
