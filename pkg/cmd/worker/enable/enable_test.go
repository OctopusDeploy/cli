package enable_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

const spaceID = "Spaces-1"

func TestWorkerEnable(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"enables a worker identified on the command line", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "enable", "Workers-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workers/Workers-100").RespondWith(fixtures.NewWorker(spaceID, "Workers-100", "build-worker", true))

			updateRequest := api.ExpectRequest(t, "PUT", "/api/Spaces-1/workers/Workers-100")
			updated, err := testutil.ReadJson[machines.Worker](updateRequest.Request.Body)
			assert.Nil(t, err)
			assert.Equal(t, false, updated.IsDisabled)
			updateRequest.RespondWith(fixtures.NewWorker(spaceID, "Workers-100", "build-worker", false))

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully enabled worker 'build-worker'")
			assert.Equal(t, "", stdErr.String())
		}},

		{"does not update a worker which is already enabled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "enable", "Workers-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workers/Workers-100").RespondWith(fixtures.NewWorker(spaceID, "Workers-100", "build-worker", false))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "is already enabled")
			assert.Equal(t, "", stdErr.String())
		}},

		{"prompts for the worker when none was supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"worker", "enable"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/workers?isDisabled=true&take=2147483647").
				RespondWith(resources.Resources[*machines.Worker]{Items: []*machines.Worker{
					fixtures.NewWorker(spaceID, "Workers-100", "build-worker", true),
					fixtures.NewWorker(spaceID, "Workers-200", "test-worker", true),
				}})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the worker you wish to enable:",
				Options: []string{"build-worker", "test-worker"},
			}).AnswerWith("build-worker")

			api.ExpectRequest(t, "PUT", "/api/Spaces-1/workers/Workers-100").RespondWith(fixtures.NewWorker(spaceID, "Workers-100", "build-worker", false))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully enabled worker 'build-worker'")
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
