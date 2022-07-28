package testutil

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/stretchr/testify/assert"
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
