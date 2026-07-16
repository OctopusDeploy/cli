package delete_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestChannelDelete(t *testing.T) {
	const spaceID = "Spaces-1"
	const projectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	hotfixChannel := fixtures.NewChannel(spaceID, "Channels-2", "Hotfix", projectID)
	hotfixChannel.Type = channels.ChannelTypeLifecycle

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"channel delete requires a project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "--project is required")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete by id with confirm flag (automation)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "-p", "Projects-22", "--no-prompt", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(hotfixChannel)
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete by name falls back to project lookup", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Hotfix", "-p", "Projects-22", "--no-prompt", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			// GetByID with a name 404s, so we fall back to the project-scoped channel list.
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Hotfix").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{hotfixChannel},
				})
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete interactive prompts for confirmation", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "-p", "Projects-22"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(hotfixChannel)

			q := qa.ExpectQuestion(t, &survey.Input{
				Message: `You are about to delete the channel "Hotfix" (Channels-2). This action cannot be reversed. To confirm, type the channel name:`,
			})
			_ = q.AnswerWith("Hotfix")

			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete warns for version-controlled (CaC) projects", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cacProject := fixtures.NewVersionControlledProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
			cacProject.IsVersionControlled = true
			cacChannel := fixtures.NewChannel(spaceID, "Channels-2", "Hotfix", projectID)
			cacChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "-p", "Projects-22", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(cacProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(cacChannel)
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Warning: This project is version-controlled (Config-as-Code). Deleting this channel can break OCL deployments that reference it.
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
