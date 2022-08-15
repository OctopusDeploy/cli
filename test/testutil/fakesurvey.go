package testutil

import (
	"errors"
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
)

type answerOrError struct {
	answer any
	error  error
}

type AskMocker struct {
	// when the client asks a question, we receive it here
	Question chan survey.Prompt
	// when we want to answer the question, we send the response here
	Answer chan answerOrError

	// so test code can detect unanswered requests or responses at the end.
	// Not strictly neccessary as unanswered req/resp results in a channel deadlock
	// and go panics and kills the process, so we find out about it, but this is a bit
	// less confusing to troubleshoot
	pendingMsgCount int32

	Closed bool
}

func (m *AskMocker) AsAsker() func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
		if m.Closed {
			return errors.New("AskMocker can't prompt; channel closed")
		}

		// we're the client here, so we send a question down the question channel
		atomic.AddInt32(&m.pendingMsgCount, 1)
		m.Question <- p
		atomic.AddInt32(&m.pendingMsgCount, -1)

		// then we wait for a response via the answer channel
		atomic.AddInt32(&m.pendingMsgCount, 1)
		x := <-m.Answer
		atomic.AddInt32(&m.pendingMsgCount, -1)

		if x.answer != nil {
			_ = core.WriteAnswer(response, "", x.answer)
		}
		return x.error
	}
}

func NewAskMocker() *AskMocker {
	return &AskMocker{
		Question: make(chan survey.Prompt),
		Answer:   make(chan answerOrError),
	}
}

func (m *AskMocker) Close() {
	m.Closed = true
	close(m.Question)
	close(m.Answer)
}

func (m *AskMocker) GetPendingMessageCount() int {
	return int(m.pendingMsgCount)
}

func (m *AskMocker) ReceiveQuestion() (survey.Prompt, bool) {
	if m.Closed {
		return nil, false
	}
	atomic.AddInt32(&m.pendingMsgCount, 1)
	request := <-m.Question
	atomic.AddInt32(&m.pendingMsgCount, -1)
	return request, !m.Closed // reading from closed channels works fine and just returns the default
}

func (m *AskMocker) SendAnswer(answer any, err error) {
	if m.Closed {
		return
	}

	atomic.AddInt32(&m.pendingMsgCount, 1)
	m.Answer <- answerOrError{answer: answer, error: err}
	atomic.AddInt32(&m.pendingMsgCount, -1)
}

// higher level niceties

func (m *AskMocker) ExpectQuestion(t *testing.T, question survey.Prompt) *QuestionWrapper {
	q, ok := m.ReceiveQuestion()
	if !ok {
		return &QuestionWrapper{Asker: m}
	}
	assert.Equal(t, question, q)
	return &QuestionWrapper{Question: q, Asker: m}
}

type QuestionWrapper struct {
	// in case you need it
	Question survey.Prompt
	Asker    *AskMocker
}

func (q *QuestionWrapper) AnswerWith(answer any) {
	q.Asker.SendAnswer(answer, nil)
}

func (q *QuestionWrapper) AnswerWithError(err error) {
	q.Asker.SendAnswer(nil, err)
}
