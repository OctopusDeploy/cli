package question_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

func TestQuestion_DeleteWithConfirmation_Success(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{
		prompt: &survey.Input{
			Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
		},
		answer: "dog",
	}})
	defer unasked()

	err := question.DeleteWithConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Nil(t, err)
}

func TestQuestion_DeleteWithConfirmation_invalidResponse(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{answer: "cat"}})
	defer unasked()
	err := question.DeleteWithConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, fmt.Errorf("input value %s does match expected value %s", "cat", "dog"))
}

func TestQuestion_DeleteWithConfirmation_error(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{err: errors.New("Ouch")}})
	defer unasked()
	err := question.DeleteWithConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, errors.New("Ouch"))
}

func TestQuestion_DeleteWithConfirmation_deleteError(t *testing.T) {
	asker, unasked := NewAskMocker(t, []QA{{answer: "dog"}})
	defer unasked()
	err := question.DeleteWithConfirmation(asker, "animal", "dog", "1", func() error { return errors.New("Ouch") })
	assert.Equal(t, err, errors.New("Ouch"))
}
