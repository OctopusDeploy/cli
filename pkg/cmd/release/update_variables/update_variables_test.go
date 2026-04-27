package update_variables_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestReleaseUpdateVariables(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	rDefault21 := fixtures.NewRelease(spaceID, "Releases-21", "2.1", fireProjectID, "Channels-1")
	rDefault20 := fixtures.NewRelease(spaceID, "Releases-20", "2.0", fireProjectID, "Channels-1")

	expectProjectLookup := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"noprompt: posts to snapshot-variables endpoint and prints success", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "update-variables", "--project", fireProject.Name, "--version", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/2.1").RespondWith(rDefault21)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-21/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot for release '2.1'")
			assert.Contains(t, stdOut.String(), "Releases-21")
			assert.NotContains(t, stdOut.String(), "Automation Command:")
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: legacy --releaseNumber alias maps to --version", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "update-variables", "--project", fireProject.Name, "--releaseNumber", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/2.1").RespondWith(rDefault21)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-21/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot for release '2.1'")
		}},

		{"noprompt: server returns non-2xx status returns wrapped error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "update-variables", "--project", fireProject.Name, "--version", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/2.1").RespondWith(rDefault21)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-21/snapshot-variables").
				RespondWithStatus(500, "500 Internal Server Error", "boom")

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update variable snapshot (HTTP 500)")
			assert.Contains(t, err.Error(), "boom")
		}},

		{"interactive: prompts for project and release then posts", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "update-variables"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project containing the release",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20},
				})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Release to Update Variables for Progression for",
				Options: []string{rDefault21.Version, rDefault20.Version},
			}).AnswerWith(rDefault21.Version)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/2.1").RespondWith(rDefault21)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-21/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot for release '2.1'")
			assert.Contains(t, stdOut.String(), "Automation Command:")
			assert.Contains(t, stdOut.String(), "--project 'Fire Project'")
			assert.Contains(t, stdOut.String(), "--version '2.1'")
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
