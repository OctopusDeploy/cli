package testutil

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
)

type PA struct {
	Prompt               survey.Prompt
	Answer               any
	ShouldSkipValidation bool
}

type CheckRemaining func()

func NewInputPrompt(prompt string, help string, response string) *PA {
	return &PA{
		Prompt: &survey.Input{
			Message: prompt,
			Help:    help,
		},
		Answer: response,
	}
}

func NewPasswordPrompt(prompt string, help string, response string) *PA {
	return &PA{
		Prompt: &survey.Password{
			Message: prompt,
			Help:    help,
		},
		Answer: response,
	}
}

func NewSelectPrompt(prompt string, help string, options []string, response string) *PA {
	return &PA{
		Prompt: &survey.Select{
			Message: prompt,
			Options: options,
			Help:    help,
		},
		Answer: response,
	}
}

func NewConfirmPrompt(prompt string, help string, response any) *PA {
	return &PA{
		Prompt: &survey.Confirm{
			Message: prompt,
			Help:    help,
		},
		Answer: response,
	}
}

func NewMockAsker(t *testing.T, pa []*PA) (question.Asker, CheckRemaining) {
	expectedQuestionIndex := 0

	checkRemaining := func() {
		if expectedQuestionIndex >= len(pa) {
			return
		}
		remainingPA := pa[expectedQuestionIndex:]
		for _, remaining := range remainingPA {
			assert.Fail(t, fmt.Sprintf("Expected the following prompt: %+v", remaining.Prompt))
		}
	}

	mockAsker := func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if expectedQuestionIndex >= len(pa) {
			assert.FailNow(t, fmt.Sprintf("Did not expect anymore questions but got: %+v", p))
			return fmt.Errorf("did not expect anymore questions")
		}

		options := &survey.AskOptions{}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(options); err != nil {
				return err
			}
		}

		if response == nil {
			return errors.New("cannot call Ask() with a nil reference to record the answers")
		}

		validate := func(q *survey.Question, val interface{}) error {
			if q.Validate != nil {
				if err := q.Validate(val); err != nil {
					return err
				}
			}
			for _, v := range options.Validators {
				if err := v(val); err != nil {
					return err
				}
			}
			return nil
		}

		expectedQA := pa[expectedQuestionIndex]
		expectedQuestionIndex += 1

		isEqual := assert.Equal(t, expectedQA.Prompt, p)
		if !isEqual {
			return fmt.Errorf("did not get expected question")
		}

		currentQuestion := survey.Question{Prompt: p}

		if !expectedQA.ShouldSkipValidation {
			validationErr := validate(&currentQuestion, expectedQA.Answer)
			if !assert.NoError(t, validationErr) {
				return validationErr
			}
		}

		if err := core.WriteAnswer(response, "", expectedQA.Answer); err != nil {
			return err
		}

		return nil
	}
	return mockAsker, checkRemaining
}

type answerOrError struct {
	answer any
	error  error
}

type questionWithOptions struct {
	question survey.Prompt
	options  *survey.AskOptions // may be nil, many things have no options
}

type AskMocker struct {
	// when the client asks a question, we receive it here
	Question chan questionWithOptions
	// when we want to answer the question, we send the response here
	Answer chan answerOrError

	Closed bool

	// when we run validators against a question, if there is an error it will be
	// sent down this channel. If you aren't hooked up to receive the validation error, the test will deadlock
	LastValidationError chan error
}

func (m *AskMocker) AsAsker() func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return func(p survey.Prompt, response any, opts ...survey.AskOpt) error {
		if m.Closed {
			return errors.New("AskMocker can't prompt; channel closed")
		}

		// we're the client here, so we send a question down the question channel
		var askOptions survey.AskOptions
		if len(opts) > 0 {
			err := opts[0](&askOptions)
			if err != nil {
				// error getting options, this shouldn't happen
				return err
			}
		}

		m.Question <- questionWithOptions{question: p, options: &askOptions}

		// then we wait for a response via the answer channel.
		// NOTE validations should have already been run on the send side, so we should only receive things
		// that have passed any survey validators. We mostly do this because the concurrent nature of this
		// makes it much easier to have the "AnswerWith" do the validation than the more correct place (here.
		x := <-m.Answer

		if x.answer != nil {
			_ = core.WriteAnswer(response, "", x.answer)
		}
		return x.error
	}
}

func NewAskMocker() *AskMocker {
	return &AskMocker{
		Question:            make(chan questionWithOptions),
		Answer:              make(chan answerOrError),
		LastValidationError: make(chan error),
	}
}

func (m *AskMocker) GetLastValidationError() error {
	return <-m.LastValidationError
}

func (m *AskMocker) Close() {
	m.Closed = true
	close(m.Question)
	close(m.Answer)
}

func (m *AskMocker) receiveQuestion() (survey.Prompt, *survey.AskOptions, bool) {
	if m.Closed {
		return nil, nil, false
	}
	request := <-m.Question
	return request.question, request.options, !m.Closed // reading from closed channels works fine and just returns the default
}

// sendAnswer blindly sends an answer down the answer channel, regardless of whether
// a question has been asked or not. You should use ExpectQuestion.AnswerWith instead of this
func (m *AskMocker) sendAnswer(answer any, err error) {
	if m.Closed {
		return
	}

	m.Answer <- answerOrError{answer: answer, error: err}
}

// ReceiveQuestion gets the next question and options from the channel and returns them in a wrapper struct.
// If the channel is closed, will still return a valid wrapper, but the wrapped contents will be nil
func (m *AskMocker) ReceiveQuestion() *QuestionWrapper {
	prompt, askOptions, ok := m.receiveQuestion()
	if !ok {
		return &QuestionWrapper{Asker: m}
	}
	return &QuestionWrapper{Question: prompt, Options: askOptions, Asker: m}
}

// ExpectQuestion calls ReceiveQuestion and asserts that the received survey prompt matches `question`
func (m *AskMocker) ExpectQuestion(t *testing.T, question survey.Prompt) *QuestionWrapper {
	q := m.ReceiveQuestion()
	assert.Equal(t, question, q.Question)
	return q
}

type QuestionWrapper struct {
	// in case you need it
	Question survey.Prompt
	Options  *survey.AskOptions
	Asker    *AskMocker
}

// AnswerWith runs any validators associated with the question. If they all pass, it sends the answer
// down the channel. If any fail, the validation error is returned from here, and the answer is NOT sent.
// This mimics the behaviour of real survey, which will keep asking you in a loop until the validators pass.
//
// If you want to test validators specifically, then do this:
//
//	q := mockSurvey.ExpectQuestion(t, &survey.Prompt{ Message: "Please input a number between 1 and 10" })
//	err := q.AnswerWith("9999")
//	assert.EqualError(t, err, "Number was not within range 1 to 10")
//	err := q.AnswerWith("-1")
//	assert.EqualError(t, err, "Number was not within range 1 to 10")
//	err := q.AnswerWith("5")
//	assert.EqualError(t, err, nil)
//	test sequence should proceed now
func (q *QuestionWrapper) AnswerWith(answer any) error {
	// run validators, otherwise we won't be able to test them
	if q.Options != nil && len(q.Options.Validators) > 0 {
		for _, validator := range q.Options.Validators {
			validationErr := validator(answer)
			if validationErr != nil {
				return validationErr
			}
		}
	}

	q.Asker.sendAnswer(answer, nil)
	return nil
}

func (q *QuestionWrapper) AnswerWithError(err error) {
	q.Asker.sendAnswer(nil, err)
}
