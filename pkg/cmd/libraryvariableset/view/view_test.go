package view_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func newVariable(id string, name string, value string, scope variables.VariableScope) *variables.Variable {
	v := variables.NewVariable(name)
	v.ID = id
	v.Value = value
	v.Scope = scope
	return v
}

func TestLibraryVariableSetView(t *testing.T) {
	const spaceID = "Spaces-1"
	const setID = "LibraryVariableSets-1"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	slackSet := fixtures.NewLibraryVariableSet(spaceID, setID, "Slack Variables")
	slackSet.Description = "Slack webhooks"

	sharedSet := fixtures.NewLibraryVariableSet(spaceID, "LibraryVariableSets-2", "Global Variables")

	// the same name stored three times: once unscoped, twice scoped
	variableSet := fixtures.NewVariableSetForLibraryVariableSet(spaceID, setID)
	variableSet.ScopeValues = &variables.VariableScopeValues{
		Environments: []*resources.ReferenceDataItem{
			{ID: "Environments-1", Name: "Production"},
			{ID: "Environments-2", Name: "Test"},
		},
	}
	sensitive := newVariable("Variables-4", "Slack.Token", "", variables.VariableScope{})
	sensitive.IsSensitive = true
	variableSet.Variables = []*variables.Variable{
		newVariable("Variables-2", "Slack.Url", "https://prod", variables.VariableScope{Environments: []string{"Environments-1"}}),
		newVariable("Variables-1", "Slack.Url", "https://default", variables.VariableScope{}),
		newVariable("Variables-3", "Slack.Url", "https://test", variables.VariableScope{Environments: []string{"Environments-2"}}),
		sensitive,
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"library variable set view requires a set in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "library variable set must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"library variable set view fails when the named set doesn't exist", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", "Nope", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "cannot find library variable set 'Nope'")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"library variable set view prompts for the set in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", "-f", "table", "-q", "token"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the library variable set you wish to view:",
				Options: []string{slackSet.Name, sharedSet.Name},
			}).AnswerWith(slackSet.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/variableset-LibraryVariableSets-1").RespondWith(variableSet)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME         VALUE  SCOPE       ID
				Slack.Token  ***    (unscoped)  Variables-4
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"library variable set view groups values sharing a name (table)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", "Slack Variables", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/variableset-LibraryVariableSets-1").RespondWith(variableSet)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME         VALUE            SCOPE                    ID
				Slack.Token  ***              (unscoped)               Variables-4
				Slack.Url    https://default  (unscoped)               Variables-1
				             https://prod     Environment: Production  Variables-2
				             https://test     Environment: Test        Variables-3
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"library variable set view by id (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", setID, "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/variableset-LibraryVariableSets-1").RespondWith(variableSet)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Slack Variables (LibraryVariableSets-1)
				Slack webhooks

				Slack.Token
				  (unscoped) = ***

				Slack.Url
				  (unscoped) = https://default
				  Environment: Production = https://prod
				  Environment: Test = https://test

				View this library variable set in Octopus Deploy: http://server/app#/Spaces-1/library/variables/LibraryVariableSets-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic still shows the web url when the filter matches nothing", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", setID, "--no-prompt", "-f", "basic", "-q", "nothing-matches-this"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/variableset-LibraryVariableSets-1").RespondWith(variableSet)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Slack Variables (LibraryVariableSets-1)
				Slack webhooks

				No variables

				View this library variable set in Octopus Deploy: http://server/app#/Spaces-1/library/variables/LibraryVariableSets-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "view", "Slack Variables", "--no-prompt", "--output-format", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/variableset-LibraryVariableSets-1").RespondWith(variableSet)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type scope struct {
				Environment []string
			}
			type value struct {
				Id           string
				Value        string
				IsSensitive  bool
				Type         string
				IsScoped     bool
				Scope        *scope
				ScopeSummary string
			}
			type group struct {
				Name   string
				Values []value
			}
			type x struct {
				Id            string
				Name          string
				Description   string
				SpaceId       string
				VariableSetId string
				TemplateCount int
				Variables     []group
				WebUrl        string
			}
			parsedStdout, err := testutil.ParseJsonStrict[x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, x{
				Id:            setID,
				Name:          "Slack Variables",
				Description:   "Slack webhooks",
				SpaceId:       spaceID,
				VariableSetId: "variableset-LibraryVariableSets-1",
				TemplateCount: 0,
				WebUrl:        "http://server/app#/Spaces-1/library/variables/LibraryVariableSets-1",
				Variables: []group{
					{Name: "Slack.Token", Values: []value{
						{Id: "Variables-4", Value: "", IsSensitive: true, Type: "String", IsScoped: false, ScopeSummary: "(unscoped)"},
					}},
					{Name: "Slack.Url", Values: []value{
						{Id: "Variables-1", Value: "https://default", Type: "String", IsScoped: false, ScopeSummary: "(unscoped)"},
						{Id: "Variables-2", Value: "https://prod", Type: "String", IsScoped: true, Scope: &scope{Environment: []string{"Production"}}, ScopeSummary: "Environment: Production"},
						{Id: "Variables-3", Value: "https://test", Type: "String", IsScoped: true, Scope: &scope{Environment: []string{"Test"}}, ScopeSummary: "Environment: Test"},
					}},
				},
			}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, qa, rootCmd, stdout, stderr)
		})
	}
}
