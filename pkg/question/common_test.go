//go:build !integration

package question_test

import (
	"errors"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

type MockAsker struct {
	OnSimpleTextQuestion func(name string, promptMessage string) (string, error)
}

func (a *MockAsker) SimpleTextQuestion(name string, promptMessage string) (string, error) {
	return a.OnSimpleTextQuestion(name, promptMessage)
}

func TestQuestion_AskForDeleteConfirmation_Success(t *testing.T) {
	asker := &MockAsker{OnSimpleTextQuestion: func(name string, promptMessage string) (string, error) {
		assert.Equal(t, "Confirm Delete", name)
		assert.Equal(t, `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`, promptMessage)

		return "dog", nil // the user typed this
	}}

	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Nil(t, err) // no error
}

func TestQuestion_AskForDeleteConfirmation_invalidResponse(t *testing.T) {
	asker := &MockAsker{OnSimpleTextQuestion: func(name string, promptMessage string) (string, error) {
		return "cat", nil // the user typed this
	}}
	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, errors.New("Canceled"))
}

func TestQuestion_AskForDeleteConfirmation_error(t *testing.T) {
	asker := &MockAsker{OnSimpleTextQuestion: func(name string, promptMessage string) (string, error) {
		return "", errors.New("Ouch")
	}}

	err := question.AskForDeleteConfirmation(asker, "animal", "dog", "1", func() error { return nil })
	assert.Equal(t, err, errors.New("Ouch"))
}
