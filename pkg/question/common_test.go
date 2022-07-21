package question_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

func TestQuestion_AskForDeleteConfirmation_Success(t *testing.T) {
	var capturedQs []*survey.Question
	question.Ask = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		capturedQs = qs
		reflect.ValueOf(response).Elem().Set(reflect.ValueOf("dog"))
		return nil
	}

	err := question.AskForDeleteConfirmation("animal", "dog", "1")
	assert.Nil(t, err) // no error

	if len(capturedQs) != 1 {
		assert.Fail(t, "Wrong number of questions")
		return
	}
	q := capturedQs[0]
	assert.Equal(t, "Confirm Delete", q.Name)
	// assert.Equal(t, "xx", q.Prompt) // We fall off the grid here because x.prompt is
	// *survey.Input(&survey.Input{Renderer:survey.Renderer{stdio:terminal.Stdio{In:terminal.FileReader(nil), Out:terminal.FileWriter(nil), Err:io.Writer(nil)}, renderedErrors:bytes.Buffer{buf:[]uint8(nil), off:0, lastRead:0}, renderedText:bytes.Buffer{buf:[]uint8(nil), off:0, lastRead:0}}, Message:"You are about to delete the animal \"dog\" (1). This action cannot be reversed. To confirm, type the animal name:"
}

func TestQuestion_AskForDeleteConfirmation_invalidResponse(t *testing.T) {
	question.Ask = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		reflect.ValueOf(response).Elem().Set(reflect.ValueOf("cat"))
		return nil
	}

	err := question.AskForDeleteConfirmation("animal", "dog", "1")
	assert.Equal(t, err, errors.New("Canceled"))
}

func TestQuestion_AskForDeleteConfirmation_error(t *testing.T) {
	question.Ask = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		return errors.New("Ouch")
	}

	err := question.AskForDeleteConfirmation("animal", "dog", "1")
	assert.Equal(t, err, errors.New("Ouch"))
}
