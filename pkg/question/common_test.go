package question_test

import (
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

func TestQuestion_AskForDeleteConfirmation_Success(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{
		prompt: &survey.Input{
			Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
		},
		answer: "dog",
	}})
	defer unasked()

	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Nil(t, err)
}

func TestQuestion_AskForDeleteConfirmation_invalidResponse(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{answer: "cat"}})
	defer unasked()
	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, errors.New("Canceled"))
}

func TestQuestion_AskForDeleteConfirmation_error(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{err: errors.New("Ouch")}})
	defer unasked()
	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, errors.New("Ouch"))
}
