package question

import (
	"github.com/AlecAivazis/survey/v2"
)

type Asker func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
