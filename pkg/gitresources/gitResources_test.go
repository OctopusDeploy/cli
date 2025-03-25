package gitresources_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/gitresources"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
)

func TestReleaseCreate_ParseGitResourceOverrideString(t *testing.T) {
	tests := []struct {
		input           string
		expect          *gitresources.GitResourceGitRef
		isErrorExpected bool
	}{
		//Valid inputs
		{input: "Action:main", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "main", GitResourceName: ""}},
		{input: "Action:*", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: ""}},
		{input: "Action=refs/heads/main", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "refs/heads/main", GitResourceName: ""}},
		{input: "Action=*", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: ""}},
		{input: "Action:Name1:main", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "main", GitResourceName: "Name1"}},
		{input: "Action:Name1:*", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: "Name1"}},
		{input: "Action=Name1=refs/heads/main", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "refs/heads/main", GitResourceName: "Name1"}},
		{input: "Action=Name1=*", expect: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: "Name1"}},
		//Mixing delimiters is NOT supported (consistent with server-side) this results in an Action name that contains an = (as : is the high preference delimiter)
		{input: "Action=Name1:*", expect: &gitresources.GitResourceGitRef{ActionName: "Action=Name1", GitRef: "*", GitResourceName: ""}},

		//Invalid inputs
		{input: "", isErrorExpected: true},
		{input: "    ", isErrorExpected: true},
		{input: "Action", isErrorExpected: true},
		{input: ":refs/heads/main", isErrorExpected: true},
		{input: "::refs/heads/main", isErrorExpected: true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result, err := gitresources.ParseGitResourceGitRefString(test.input)
			assert.Equal(t, test.isErrorExpected, err != nil)
			assert.Equal(t, test.expect, result)
		})
	}
}

func TestReleaseCreate_ToGitResourceGitRefString(t *testing.T) {
	tests := []struct {
		name   string
		input  *gitresources.GitResourceGitRef
		expect string
	}{
		{name: "primary git resource", input: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "refs/heads/main", GitResourceName: ""}, expect: "Action:refs/heads/main"},
		{name: "primary git resource with wildcard", input: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: ""}, expect: "Action:*"},
		{name: "secondary git resource", input: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "refs/heads/main", GitResourceName: "Name1"}, expect: "Action:Name1:refs/heads/main"},
		{name: "secondary git resource with wildcard", input: &gitresources.GitResourceGitRef{ActionName: "Action", GitRef: "*", GitResourceName: "Name1"}, expect: "Action:Name1:*"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.input.ToGitResourceGitRefString()
			assert.Equal(t, test.expect, result)
		})
	}

}

func TestReleaseCreate_ResolveGitResourceOverride(t *testing.T) {
	baseline := []*gitresources.GitResourceGitRef{
		{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""},
		{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
		{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
		{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
	}

	tests := []struct {
		name            string
		input           *gitresources.GitResourceGitRef
		expect          *gitresources.GitResourceGitRef
		isErrorExpected bool
	}{
		//matching cases
		{name: "matches primary git resource",
			input:  &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: ""},
			expect: &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: ""}},

		{name: "matches secondary git resource",
			input:  &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: "Name1"},
			expect: &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: "Name1"}},

		{name: "matches primary git resource with wildcard",
			input:  &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "*", GitResourceName: ""},
			expect: &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""}},

		{name: "matches secondary git resource with wildcard",
			input:  &gitresources.GitResourceGitRef{ActionName: "Action3", GitRef: "*", GitResourceName: "Name1"},
			expect: &gitresources.GitResourceGitRef{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"}},

		//non-matching cases
		{name: "does not match secondary git resource by action name",
			input:           &gitresources.GitResourceGitRef{ActionName: "Action2", GitRef: "*", GitResourceName: "Name1"},
			isErrorExpected: true,
		},
		{name: "does not match secondary git resource by git resource name",
			input:           &gitresources.GitResourceGitRef{ActionName: "Action1", GitRef: "*", GitResourceName: "Name2"},
			isErrorExpected: true,
		},
		{name: "does not match primary git resource",
			input:           &gitresources.GitResourceGitRef{ActionName: "Action3", GitRef: "*", GitResourceName: ""},
			isErrorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := gitresources.ResolveGitResourceOverride(test.input, baseline)
			assert.Equal(t, test.isErrorExpected, err != nil)
			assert.Equal(t, test.expect, result)

			//we always want to validate that the result object is a new object than the input
			assert.NotSame(t, test.input, result)
		})
	}
}

func TestReleaseCreate_ApplyGitResourceOverrides(t *testing.T) {
	baseline := []*gitresources.GitResourceGitRef{
		{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""},
		{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
		{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
		{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
	}

	t.Run("no overrides results in new copied objects", func(t *testing.T) {
		var overrides []*gitresources.GitResourceGitRef

		result := gitresources.ApplyGitResourceOverrides(baseline, overrides)

		assert.Equal(t, []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""},
			{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
			{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
			{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
		}, result)

		for i := range result {
			assert.Equal(t, baseline[i], result[i])
			assert.NotSame(t, baseline[i], result[i])
		}
	})

	t.Run("applies specified overrides", func(t *testing.T) {
		overrides := []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: ""},
			{ActionName: "Action3", GitRef: "refs/tags/v1.0.0", GitResourceName: "Name1"},
		}

		result := gitresources.ApplyGitResourceOverrides(baseline, overrides)

		assert.Equal(t, []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: ""},
			{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
			{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
			{ActionName: "Action3", GitRef: "refs/tags/v1.0.0", GitResourceName: "Name1"},
		}, result)
	})

	t.Run("applies specified overrides with wildcards", func(t *testing.T) {
		overrides := []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "*", GitResourceName: ""},
		}

		result := gitresources.ApplyGitResourceOverrides(baseline, overrides)

		assert.Equal(t, []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""},
			{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
			{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
			{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
		}, result)
	})

	t.Run("only applies matching overrides", func(t *testing.T) {
		overrides := []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: ""},
			{ActionName: "Action2", GitRef: "*", GitResourceName: "Name1"},
		}

		result := gitresources.ApplyGitResourceOverrides(baseline, overrides)

		assert.Equal(t, []*gitresources.GitResourceGitRef{
			{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: ""},
			{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
			{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
			{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
		}, result)
	})
}

func TestReleaseCreate_AskQuestions_AskGitResourceOverrideLoop(t *testing.T) {
	baseline := []*gitresources.GitResourceGitRef{
		{ActionName: "Action1", GitRef: "refs/heads/main", GitResourceName: ""},
		{ActionName: "Action1", GitRef: "refs/tags/test", GitResourceName: "Name1"},
		{ActionName: "Action2", GitRef: "release/v1", GitResourceName: ""},
		{ActionName: "Action3", GitRef: "refs/tags/v1", GitResourceName: "Name1"},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		// this is the happy path where the CLI presents the list of server-selected git resources and they just go 'yep'
		{"no-op test", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			//nothing was overridden, so an empty array
			assert.Equal(t, make([]*gitresources.GitResourceGitRef, 0), overrides)
		}},

		{"override primary git resource", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:refs/heads/elephant")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: ""},
			}, overrides)
		}},

		{"override secondary git resource", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:Name1:refs/heads/elephant")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: "Name1"},
			}, overrides)
		}},

		{"multiple overrides of same git resource results in last override used", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:refs/heads/elephant")
			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:refs/heads/dinosaur")
			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:refs/heads/lion")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/heads/lion", GitResourceName: ""},
			}, overrides)
		}},

		{"entering the loop with --git-resource picked up from the command line", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			cmdlineGitResources := []string{"Action1:Name1:refs/tags/1.0.0", "Action2:refs/heads/abc123"}

			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, cmdlineGitResources, qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: "Name1"},
				{ActionName: "Action2", GitRef: "refs/heads/abc123", GitResourceName: ""},
			}, overrides)
		}},

		{"blank answer retries the question", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			validationErr := qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("")
			assert.Nil(t, validationErr)

			validationErr = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("")
			assert.Nil(t, validationErr)

			validationErr = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")
			assert.Nil(t, validationErr)

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, make([]*gitresources.GitResourceGitRef, 0), overrides)
		}},

		{"can't specify garbage; question loop retries", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion})

			validationErr := q.AnswerWith("fish") // not enough components
			assert.EqualError(t, validationErr, "git resource git reference specification \"fish\" does not use an expected format")

			validationErr = q.AnswerWith("z:z:z:z") // too many components
			assert.EqualError(t, validationErr, "git resource git reference specification \"z:z:z:z\" does not use an expected format")

			validationErr = q.AnswerWith("refs/heads/main") // can't just have a git ref with no :
			assert.EqualError(t, validationErr, "git resource git reference specification \"refs/heads/main\" does not use an expected format")

			validationErr = q.AnswerWith("Action1:refs/heads/elephant") // answer properly this time
			assert.Nil(t, validationErr)

			// it'll ask again; y to confirm
			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y") // confirm packages

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: ""},
			}, overrides)
		}},

		{"can't specify git resources or steps that aren't there due to validator; question loop retries", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion})

			validationErr := q.AnswerWith("banana:refs/heads/main")
			assert.EqualError(t, validationErr, "could not resolve step name \"banana\" or git resource name \"\"")

			validationErr = q.AnswerWith("Action1:Name3:refs/heads/main")
			assert.EqualError(t, validationErr, "could not resolve step name \"Action1\" or git resource name \"Name3\"")

			validationErr = q.AnswerWith("Action1:refs/heads/elephant") // ok answer properly this time, set everything to 2.5
			assert.Nil(t, validationErr)

			// it'll ask again; y to confirm
			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y") // confirm packages

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/heads/elephant", GitResourceName: ""},
			}, overrides)
		}},

		{"question loop doesn't retry if it gets a hard error", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			expectedErr := errors.New("hard fail")

			qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWithError(expectedErr)

			overrides, err := testutil.ReceivePair(receiver)
			assert.Equal(t, expectedErr, err)
			assert.Nil(t, overrides)
		}},

		{"multiple overrides with undo", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:Name1:refs/tags/1.0.0")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action2:refs/tags/1.0.0")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("u") // undo Action2:refs/tags/1.0.0

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action2:refs/heads/abc123")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action1", GitRef: "refs/tags/1.0.0", GitResourceName: "Name1"},
				{ActionName: "Action2", GitRef: "refs/heads/abc123", GitResourceName: ""},
			}, overrides)
		}},

		{"multiple overrides with reset", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin2(func() ([]*gitresources.GitResourceGitRef, error) {
				return gitresources.AskGitResourceOverrideLoop(baseline, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action1:Name1:refs/tags/1.0.0")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action2:refs/tags/1.0.0")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("r") // undo Action1:Name1:refs/tags/1.0.0 and Action2:refs/tags/1.0.0

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("Action3:Name1:refs/heads/abc123")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: gitresources.GitResourceOverrideQuestion}).AnswerWith("y")

			overrides, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*gitresources.GitResourceGitRef{
				{ActionName: "Action3", GitRef: "refs/heads/abc123", GitResourceName: "Name1"},
			}, overrides)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			qa := testutil.NewAskMocker()
			test.run(t, qa, &bytes.Buffer{})
		})
	}
}
