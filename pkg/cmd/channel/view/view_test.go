package view_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdChannelView "github.com/OctopusDeploy/cli/pkg/cmd/channel/view"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
		{"channel view requires a project in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-2", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view prompts for project in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-2", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project containing the channel you wish to view",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

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

		{"channel view default channel shows inherited lifecycle (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// The default channel has no LifecycleId of its own; it inherits the project's lifecycle.
			defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Default", projectID)
			defaultChannel.IsDefault = true
			defaultChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-1", "-p", "Projects-22", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-1").RespondWith(defaultChannel)
			// No lifecycle lookup is made because LifecycleId is empty.

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Default (Channels-1)
				Default channel
				Type: Lifecycle
				Lifecycle: Inherited from project
				No description provided

				View this channel in Octopus Deploy: http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-1

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

		{"channel view will not show a channel belonging to a different project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// Channels-99 exists, but belongs to Projects-99, not the project we asked about.
			// Showing it would also render a web URL under the wrong project slug.
			foreignChannel := fixtures.NewChannel(spaceID, "Channels-99", "Secret", "Projects-99")
			foreignChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-99", "-p", "Projects-22", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-99").RespondWith(foreignChannel)
			// Rejected on project mismatch, so we fall back to the project-scoped lookup, which
			// only holds the channels that really belong to Projects-22.
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "cannot find a channel in project 'Fire Project' with the ID or name of 'Channels-99'")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view returns an error when the channel does not exist", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Nonexistent", "-p", "Projects-22", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Nonexistent").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{hotfixChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "cannot find a channel in project 'Fire Project' with the ID or name of 'Nonexistent'")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view shows tenant tags and rule counts (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			richChannel := fixtures.NewChannel(spaceID, "Channels-3", "Tenanted", projectID)
			richChannel.Description = "Everything turned on"
			richChannel.LifecycleID = "Lifecycles-1"
			richChannel.Type = channels.ChannelTypeLifecycle
			richChannel.TenantTags = []string{"tagset/tag-a", "tagset/tag-b"}
			richChannel.Rules = []channels.ChannelRule{
				{ID: "rule-1", VersionRange: "[1.0,2.0)"},
				{ID: "rule-2", VersionRange: "[2.0,3.0)"},
			}
			richChannel.GitReferenceRules = []string{"refs/heads/main"}
			richChannel.GitResourceRules = []channels.ChannelGitResourceRule{{Id: "git-resource-1"}}
			richChannel.CustomFieldDefinitions = []channels.ChannelCustomFieldDefinition{
				{FieldName: "Ticket", Description: "Change ticket"},
			}

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-3", "-p", "Projects-22", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-3").RespondWith(richChannel)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Tenanted (Channels-3)
				Type: Lifecycle
				Lifecycle: Default Lifecycle (Lifecycles-1)
				Everything turned on
				Tenant tags: tagset/tag-a, tagset/tag-b
				Version rules: 2
				Git reference rules: 1
				Git resource rules: 1
				Custom field definitions: 1

				View this channel in Octopus Deploy: http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-3

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view shows ephemeral environment settings (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			ephemeralChannel := fixtures.NewEphemeralChannel(spaceID, "Channels-4", "Preview", projectID, "preview-#{Octopus.Release.Number}", true)
			ephemeralChannel.LifecycleID = "Lifecycles-1"
			ephemeralChannel.ParentEnvironmentID = "Environments-1"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-4", "-p", "Projects-22", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-4").RespondWith(ephemeralChannel)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Preview (Channels-4)
				Type: EphemeralEnvironment
				Lifecycle: Default Lifecycle (Lifecycles-1)
				No description provided
				Parent environment: Environments-1
				Ephemeral environment name template: preview-#{Octopus.Release.Number}
				Automatic deployments: true

				View this channel in Octopus Deploy: http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-4

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"channel view default channel shows inherited lifecycle (table)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Default", projectID)
			defaultChannel.IsDefault = true
			defaultChannel.Type = channels.ChannelTypeLifecycle

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"channel", "view", "Channels-1", "-p", "Projects-22", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-1").RespondWith(defaultChannel)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME     TYPE       DEFAULT  LIFECYCLE               DESCRIPTION              WEB URL
				Default  Lifecycle  yes      Inherited from project  No description provided  http://server/app#/Spaces-1/projects/fire-project/deployments/channels/edit/Channels-1
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
