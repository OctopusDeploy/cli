package testutil

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
)

type QA struct {
	Prompt survey.Prompt
	Answer any
	Err    error
}

func (qa *QA) String() string {
	return fmt.Sprintf("{\n"+
		"	prompt: %+v\n"+
		"	answer: %v\n"+
		"	err: %v"+
		"\n}\n", qa.Prompt, qa.Answer, qa.Err)
}

func NewAskMocker(t assert.TestingT, questions []QA) (question.Asker, func()) {
	c := make(chan QA)
	go func() {
		for _, qa := range questions {
			c <- qa
		}
		close(c)
	}()
	return func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
			qa, more := <-c
			if !more {
				assert.Fail(t, fmt.Sprintf("Unexpected question asked: \n"+
					"prompt: %+v", p))
			}
			if qa.Prompt != nil {
				assert.Equal(t, qa.Prompt, p)
			}
			if qa.Answer != nil {
				_ = core.WriteAnswer(response, "", qa.Answer)
			}
			return qa.Err
		},
		func() {
			extraQuestions := make([]string, 0)
			for {
				qa, more := <-c
				if !more {
					if len(extraQuestions) > 0 {
						assert.Fail(t, fmt.Sprintf("Expected the following question(s) to be asked: \n"+
							"questions: %v", extraQuestions))
					}
					return
				}
				extraQuestions = append(extraQuestions, qa.String())
			}
		}
}

func NewAskOneMocker(t assert.TestingT, question QA) (question.Asker, func()) {
	return NewAskMocker(t, []QA{question})
}

// ---------------------------------------------------------------------------------------------------------------------
// NEW PROTOTYPE STUFF
// ---------------------------------------------------------------------------------------------------------------------

type answerOrError struct {
	answer any
	error  error
}

type AskMocker2 struct {
	// when the client asks a question, we receive it here
	Question chan survey.Prompt
	// when we want to answer the question, we send the response here
	Answer chan answerOrError

	// so test code can detect unanswered requests or responses at the end.
	// Not strictly neccessary as unanswered req/resp results in a channel deadlock
	// and go panics and kills the process, so we find out about it, but this is a bit
	// less confusing to troubleshoot
	pendingMsgCount int32
}

func (m *AskMocker2) AsAsker() func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
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

func NewAskMocker2() *AskMocker2 {
	return &AskMocker2{
		Question: make(chan survey.Prompt),
		Answer:   make(chan answerOrError),
	}
}

func (m *AskMocker2) GetPendingMessageCount() int {
	return int(m.pendingMsgCount)
}

func (m *AskMocker2) ReceiveQuestion() survey.Prompt {
	atomic.AddInt32(&m.pendingMsgCount, 1)
	request := <-m.Question
	atomic.AddInt32(&m.pendingMsgCount, -1)
	return request
}

func (m *AskMocker2) SendAnswer(answer any, err error) {
	atomic.AddInt32(&m.pendingMsgCount, 1)
	m.Answer <- answerOrError{answer: answer, error: err}
	atomic.AddInt32(&m.pendingMsgCount, -1)
}

// higher level niceties

func (m *AskMocker2) ExpectQuestion(t *testing.T, question survey.Prompt) *QuestionWrapper {
	q := m.ReceiveQuestion()
	assert.Equal(t, question, q)
	return &QuestionWrapper{Question: q, Asker: m}
}

type QuestionWrapper struct {
	// in case you need it
	Question survey.Prompt
	Asker    *AskMocker2
}

func (q *QuestionWrapper) AnswerWith(answer any) {
	q.Asker.SendAnswer(answer, nil)
}
