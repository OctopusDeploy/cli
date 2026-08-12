package delete_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
		{"channel delete requires a project in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete prompts for project in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-2", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project containing the channel you wish to delete",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(hotfixChannel)
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
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

		{"channel delete will not delete a channel belonging to a different project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// Channels-99 exists, but belongs to Projects-99. It must not be deleted just
			// because the caller named a project they do have access to.
			foreignChannel := fixtures.NewChannel(spaceID, "Channels-99", "Secret", "Projects-99")
			foreignChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "Channels-99", "-p", "Projects-22", "--no-prompt", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-99").RespondWith(foreignChannel)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{hotfixChannel},
				})

			// No DELETE request is expected; api.Close() asserts nothing further was requested.
			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "no channel found with name of Channels-99")

			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete prompts to select a channel when none is given", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Default", projectID)
			defaultChannel.IsDefault = true
			defaultChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "-p", "Projects-22", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel you wish to delete:",
				Options: []string{"Default", "Hotfix"},
			}).AnswerWith("Hotfix")

			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/channels/Channels-2").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete errors when the project has no channels", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "-p", "Projects-22", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "Select the channel you wish to delete: - no options available")

			assert.Equal(t, "", stdErr.String())
		}},

		{"channel delete requires a channel in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "delete", "-p", "Projects-22", "--no-prompt", "-y"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// the flags are validated up front, so no project lookup is made
			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "channel must be specified")

			assert.Equal(t, "", stdOut.String())
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
