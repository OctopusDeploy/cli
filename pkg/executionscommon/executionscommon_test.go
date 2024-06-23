package executionscommon_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAskVariables(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	makeVariables := func(variables ...*variables.Variable) *variables.VariableSet {
		vars := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
		vars.ID = fmt.Sprintf("%s-s-0-2ZFWS", vars.ID)
		vars.Variables = variables
		return vars
	}

	tests := []struct {
		name string
		run  func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"doesn't do anything if there are no variables", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			output, err := executionscommon.AskVariables(qa.AsAsker(), makeVariables(), make(map[string]string, 0))
			assert.Nil(t, err)
			assert.Equal(t, make(map[string]string, 0), output)
		}},

		{"variablesFromCmd are filtered and normalized against the server", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			serverVars := makeVariables(variables.NewVariable("Foo"))
			cmdlineVars := map[string]string{"foO": "bar", "doesntexist": "value"}

			output, err := executionscommon.AskVariables(qa.AsAsker(), serverVars, cmdlineVars)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Foo": "bar"}, output)
		}},

		{"prompts for a single line text", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter some text",
				DisplaySettings: nil,
				IsRequired:      false,
				Label:           "ignored",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "SomeText (Enter some text)",
				Default: "",
			}).AnswerWith("Some Value")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "Some Value"}, output)
		}},

		{"single line text with default value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Value = "Some Default Value"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter some text",
				DisplaySettings: nil,
				IsRequired:      false,
				Label:           "ignored",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "SomeText (Enter some text)",
				Default: "Some Default Value",
			}).AnswerWith("Some Default Value")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "Some Default Value"}, output)
		}},

		{"prompts for a single line text with explicit display settings and required", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("ReqText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter required text",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeSingleLineText, nil),
				IsRequired:      true,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: "ReqText (Enter required text)", Default: ""})
			validationErr := q.AnswerWith("")
			assert.EqualError(t, validationErr, "Value is required")

			validationErr = q.AnswerWith("A value")
			assert.Nil(t, validationErr)

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"ReqText": "A value"}, output)
		}},

		{"prompts for a sensitive value (isSensitive on variable declaration)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.IsSensitive = true
			v1.Prompt = &variables.VariablePromptOptions{Description: "Enter secret text"}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		{"prompts for a sensitive value (controlType=sensitive)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter secret text",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeSensitive, nil),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		{"prompts for a sensitive value (variable.type=sensitive)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Type = "Sensitive"
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Enter secret text",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		// REFER: https://github.com/OctopusDeploy/Issues/issues/7699
		{"does not prompt for complex variable types", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			// It is possible in octopus to set the 'Prompt' flag on complex variable types such as accounts and certificates.
			// This results in the web portal prompting for user input, but this doesn't work as expected.
			// As at 5 Sep 2022, the FNM team hadn't decided how they were going to fix it. Interim measure is to
			// have the CLI ignore the prompt flag on these kinds of variables and revisit later once FNM have resolved the bug
			v1 := variables.NewVariable("Certificate")
			v1.Type = "Certificate"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Codesigning certificate?",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeCertificate, nil),
			}

			v2 := variables.NewVariable("AWS Account")
			v2.Type = "AzureAccount"
			v2.Prompt = &variables.VariablePromptOptions{
				Description:     "AZ Account?",
				DisplaySettings: &resources.DisplaySettings{},
			}
			vars := makeVariables(v1, v2)

			output, err := executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))

			assert.Nil(t, err)
			assert.Equal(t, make(map[string]string, 0), output)

		}},

		{"prompts for a combo box value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("Division")
			v1.Type = "String"
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Which part of the company do you work in?",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeSelect, []*resources.SelectOption{
					{Value: "rnd", DisplayName: "R&D"},
					{Value: "finance", DisplayName: "Finance"},
					{Value: "hr", DisplayName: "Human Resources"},
					{Value: "other", DisplayName: "Other"},
				}),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Division (Which part of the company do you work in?)",
				Options: []string{"R&D", "Finance", "Human Resources", "Other"},
				Default: "",
			}).AnswerWith("Human Resources")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Division": "hr"}, output)
		}},

		{"combo box with default value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("Division")
			v1.Type = "String"
			v1.Value = "rnd" // looks this up!
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Which part of the company do you work in?",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeSelect, []*resources.SelectOption{
					{Value: "rnd", DisplayName: "R&D"},
					{Value: "finance", DisplayName: "Finance"},
					{Value: "hr", DisplayName: "Human Resources"},
					{Value: "other", DisplayName: "Other"},
				}),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Division (Which part of the company do you work in?)",
				Options: []string{"R&D", "Finance", "Human Resources", "Other"},
				Default: "R&D",
			}).AnswerWith("Other")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Division": "other"}, output)
		}},

		{"checkbox", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("IsApproved")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Is this approved?",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeCheckbox, nil),
				IsRequired:      false,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "IsApproved (Is this approved?)",
				Default: "False", // checkbox defaults itself to false if not specified
				Options: []string{"True", "False"},
			}).AnswerWith("True")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"IsApproved": "True"}, output)
		}},

		{"checkbox with default value true", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("IsApproved")
			v1.Value = "True"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Is this approved?",
				DisplaySettings: resources.NewDisplaySettings(resources.ControlTypeCheckbox, nil),
				IsRequired:      false,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return executionscommon.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "IsApproved (Is this approved?)",
				Default: "True",
				Options: []string{"True", "False"},
			}).AnswerWith("True")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"IsApproved": "True"}, output)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			qa := testutil.NewAskMocker()
			test.run(t, qa, new(bytes.Buffer))
		})
	}
}

func TestParseVariableStringArray(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expect    map[string]string
		expectErr error
	}{
		{name: "foo:bar", input: []string{"foo:bar"}, expect: map[string]string{"foo": "bar"}},
		{name: "foo:bar,baz:qux", input: []string{"foo:bar", "baz:qux"}, expect: map[string]string{"foo": "bar", "baz": "qux"}},
		{name: "foo=bar,baz=qux", input: []string{"foo=bar", "baz=qux"}, expect: map[string]string{"foo": "bar", "baz": "qux"}},

		{name: "foo:bar:more=stuff", input: []string{"foo:bar:more=stuff"}, expect: map[string]string{"foo": "bar:more=stuff"}},

		{name: "trims whitespace", input: []string{" foo : \tbar "}, expect: map[string]string{"foo": "bar"}},

		// error cases
		{name: "blank", input: []string{""}, expectErr: errors.New("could not parse variable definition ''")},
		{name: "no delimeter", input: []string{"zzz"}, expectErr: errors.New("could not parse variable definition 'zzz'")},
		{name: "missing key", input: []string{":bar"}, expectErr: errors.New("could not parse variable definition ':bar'")},
		{name: "missing val", input: []string{"foo:"}, expectErr: errors.New("could not parse variable definition 'foo:'")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := executionscommon.ParseVariableStringArray(test.input)
			assert.Equal(t, test.expectErr, err)
			assert.Equal(t, test.expect, result)
		})
	}
}

func TestToVariableStringArray(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		expect []string
	}{
		{name: "foo:bar", input: map[string]string{"foo": "bar"}, expect: []string{"foo:bar"}},

		{name: "foo:bar:more=stuff", input: map[string]string{"foo": "bar:more=stuff"}, expect: []string{"foo:bar:more=stuff"}},

		{name: "strips empty keys", input: map[string]string{"": "bar"}, expect: []string{}},
		{name: "strips empty values", input: map[string]string{"foo": ""}, expect: []string{}},

		// these two tests in combination check that output order is deterministic
		{name: "foo:bar,baz:qux", input: map[string]string{"foo": "bar", "baz": "qux"}, expect: []string{"baz:qux", "foo:bar"}},
		{name: "baz:qux,foo:bar", input: map[string]string{"baz": "qux", "foo": "bar"}, expect: []string{"baz:qux", "foo:bar"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := executionscommon.ToVariableStringArray(test.input)
			assert.Equal(t, test.expect, result)
		})
	}
}
