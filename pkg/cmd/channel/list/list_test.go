package list_test

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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestChannelList(t *testing.T) {
	const spaceID = "Spaces-1"
	const projectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Default", projectID)
	defaultChannel.IsDefault = true
	defaultChannel.LifecycleID = "Lifecycles-1"
	defaultChannel.Type = channels.ChannelTypeLifecycle

	hotfixChannel := fixtures.NewChannel(spaceID, "Channels-2", "Hotfix", projectID)
	hotfixChannel.Description = "Urgent fixes"
	hotfixChannel.LifecycleID = "Lifecycles-1"
	hotfixChannel.Type = channels.ChannelTypeLifecycle

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"channel list requires a project in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel list prompts for project in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to list channels for",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME     TYPE       DEFAULT  LIFECYCLE ID
				Default  Lifecycle  *        Lifecycles-1
				Hotfix   Lifecycle           Lifecycles-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel list picks up project from args in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "Projects-22", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME     TYPE       DEFAULT  LIFECYCLE ID
				Default  Lifecycle  *        Lifecycles-1
				Hotfix   Lifecycle           Lifecycles-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel list picks up project from flag and filters by partial name", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "-p", "Projects-22", "--partial-name", "hot", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME    TYPE       DEFAULT  LIFECYCLE ID
				Hotfix  Lifecycle           Lifecycles-1
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "-p", "Projects-22", "--output-format", "json", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				ID          string
				Name        string
				Description string
				LifecycleID string
				IsDefault   bool
				Type        string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{
				{ID: "Channels-1", Name: "Default", Description: "", LifecycleID: "Lifecycles-1", IsDefault: true, Type: "Lifecycle"},
				{ID: "Channels-2", Name: "Hotfix", Description: "Urgent fixes", LifecycleID: "Lifecycles-1", IsDefault: false, Type: "Lifecycle"},
			}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "list", "-p", "Projects-22", "--output-format", "basic", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Default
				Hotfix
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
