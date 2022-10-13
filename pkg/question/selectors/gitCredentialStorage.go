package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
)

func GitCredentialStorage(questionText string, ask question.Asker) (string, error) {
	options := []*SelectOption[string]{
		{Display: "Project", Value: "project"},
		{Display: "Library", Value: "library"},
	}

	optionsCallback := func() ([]*SelectOption[string], error) {
		return options, nil
	}

	selectedOption, err := Select(ask, questionText, optionsCallback, func(option *SelectOption[string]) string {
		return option.Display
	})

	if err != nil {
		return "", err
	}

	return selectedOption.Value, nil
}
