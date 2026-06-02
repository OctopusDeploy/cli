package view_test

import (
	"bytes"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdChannelView "github.com/OctopusDeploy/cli/pkg/cmd/channel/view"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestChannelView(t *testing.T) {
	const spaceID = "Spaces-1"
	const projectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	fireProject.Slug = "fire-project"

	lifecycle := lifecycles.NewLifecycle("Default Lifecycle")
	lifecycle.ID = "Lifecycles-1"

	hotfixChannel := fixtures.NewChannel(spaceID, "Channels-2", "Hotfix", projectID)
	hotfixChannel.Description = "Urgent fixes"
	hotfixChannel.LifecycleID = "Lifecycles-1"
	hotfixChannel.Type = channels.ChannelTypeLifecycle

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"channel view requires a project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-2"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "--project is required")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view by id (table)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-2", "-p", "Projects-22", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(hotfixChannel)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME    TYPE       DEFAULT  LIFECYCLE          DESCRIPTION   WEB URL
				Hotfix  Lifecycle           Default Lifecycle  Urgent fixes  http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-2
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view by name falls back to project lookup (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Hotfix", "-p", "Projects-22", "-f", "basic"})
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
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Hotfix (Channels-2)
				Type: Lifecycle
				Lifecycle: Default Lifecycle (Lifecycles-1)
				Urgent fixes

				View this channel in Octopus Deploy: http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-2

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-2", "-p", "Projects-22", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-2").RespondWith(hotfixChannel)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			parsedStdout, err := testutil.ParseJsonStrict[cmdChannelView.ChannelAsJson](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, "Channels-2", parsedStdout.Id)
			assert.Equal(t, "Hotfix", parsedStdout.Name)
			assert.Equal(t, "Urgent fixes", parsedStdout.Description)
			assert.Equal(t, "Projects-22", parsedStdout.ProjectId)
			assert.Equal(t, "Lifecycles-1", parsedStdout.LifecycleId)
			assert.Equal(t, "Default Lifecycle", parsedStdout.LifecycleName)
			assert.Equal(t, "Lifecycle", parsedStdout.Type)
			assert.False(t, parsedStdout.IsDefault)
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
