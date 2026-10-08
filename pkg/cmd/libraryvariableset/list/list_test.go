package list_test

import (
	"bytes"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestLibraryVariableSetList(t *testing.T) {
	const spaceID = "Spaces-1"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	slackSet := fixtures.NewLibraryVariableSet(spaceID, "LibraryVariableSets-1", "Slack Variables")
	slackSet.Description = "Slack webhooks"

	sharedSet := fixtures.NewLibraryVariableSet(spaceID, "LibraryVariableSets-2", "Global Variables")

	scriptModule := fixtures.NewLibraryVariableSet(spaceID, "LibraryVariableSets-3", "Helper Functions")
	scriptModule.ContentType = "ScriptModule"

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"library variable set list prints the sets, sorted, excluding script modules", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, scriptModule, sharedSet})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME              DESCRIPTION     ID
				Global Variables                  LibraryVariableSets-2
				Slack Variables   Slack webhooks  LibraryVariableSets-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"library variable set list filters by name", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "ls", "-q", "slack", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME             DESCRIPTION     ID
				Slack Variables  Slack webhooks  LibraryVariableSets-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "list", "--output-format", "json", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				ID            string
				Name          string
				Description   string
				VariableSetID string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{
				{ID: "LibraryVariableSets-2", Name: "Global Variables", Description: "", VariableSetID: "variableset-LibraryVariableSets-2"},
				{ID: "LibraryVariableSets-1", Name: "Slack Variables", Description: "Slack webhooks", VariableSetID: "variableset-LibraryVariableSets-1"},
			}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"library-variable-set", "list", "--output-format", "basic", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/libraryvariablesets/all").
				RespondWith([]*variables.LibraryVariableSet{slackSet, sharedSet})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Global Variables
				Slack Variables
				`), stdOut.String())
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
