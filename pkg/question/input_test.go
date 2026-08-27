package question_test

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"testing"

	"github.com/OctopusDeploy/cli/test/testutil"

	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

func TestQuestion_DeleteWithConfirmation_Success(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return nil })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWith("dog")

	err := <-errReceiver
	assert.Nil(t, err)
}

func TestQuestion_DeleteWithConfirmation_invalidResponse(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return nil })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWith("cat")

	err := <-errReceiver
	assert.Equal(t, err, fmt.Errorf("input value %s does match expected value %s", "cat", "dog"))
}

func TestQuestion_DeleteWithConfirmation_error(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return nil })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWithError(errors.New("ouch"))

	err := <-errReceiver
	assert.Equal(t, errors.New("ouch"), err)
}

// Confirming a deletion means retyping the item's name, which cannot happen with
// prompting disabled. On its own that surfaced only "prompt disabled", with no
// hint that --confirm is the way to delete without being asked.
func TestQuestion_DeleteWithConfirmation_promptDisabled(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return nil })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWithError(&cliErrors.PromptDisabledError{})

	err := <-errReceiver
	assert.EqualError(t, err,
		"cannot confirm deletion of the animal while prompting is disabled; pass --confirm to delete it without confirmation")
}

func TestQuestion_DeleteWithConfirmation_wrappedPromptDisabled(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return nil })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWithError(errors.Join(errors.New("while deleting"), &cliErrors.PromptDisabledError{}))

	err := <-errReceiver
	assert.EqualError(t, err,
		"cannot confirm deletion of the animal while prompting is disabled; pass --confirm to delete it without confirmation")
}

func TestQuestion_DeleteWithConfirmation_deleteError(t *testing.T) {
	qa := testutil.NewAskMocker()
	errReceiver := testutil.GoBegin(func() error {
		return question.DeleteWithConfirmation(qa.AsAsker(), "animal", "dog", "1", func() error { return errors.New("ouch") })
	})

	qa.ExpectQuestion(t, &survey.Input{
		Message: `You are about to delete the animal "dog" (1). This action cannot be reversed. To confirm, type the animal name:`,
	}).AnswerWith("dog")

	err := <-errReceiver
	assert.Equal(t, errors.New("ouch"), err)
}

func TestAskName(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("prefix Name", "A short, memorable, unique name for this resource.", "answer"),
	}
	qa, _ := testutil.NewMockAsker(t, pa)

	var value string
	err := question.AskName(qa, "prefix ", "resource", &value)
	assert.NoError(t, err)
	assert.Equal(t, value, "answer")
}
