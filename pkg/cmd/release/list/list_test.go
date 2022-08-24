package list_test

import (
	"bytes"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
const packageOverrideQuestion = "Package override string (y to accept, u to undo, ? for help):"

var spinner = &testutil.FakeSpinner{}

var rootResource = testutil.NewRootResource()

// if this were bigger we'd split out the AskQuestions into a seperate test group, but because there's only one we don't worry about it
func TestReleaseList(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Default Channel", fireProjectID)
	betaChannel := fixtures.NewChannel(spaceID, "Channels-13", "Beta Channel", fireProjectID)

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"release list requires a project name in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release list prompts for project name in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to list releases for",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{
						releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1"),
						releases.NewRelease(defaultChannel.ID, fireProjectID, "2.0"),
						releases.NewRelease(betaChannel.ID, fireProjectID, "2.0-beta2"),
						releases.NewRelease(betaChannel.ID, fireProjectID, "2.0-beta1"),
					},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1%2CChannels-13&take=2").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, betaChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				VERSION    CHANNEL
				2.1        Default Channel
				2.0        Default Channel
				2.0-beta2  Beta Channel
				2.0-beta1  Beta Channel
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release list picks up project from args in automation mode and prints list with multiple channels", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", fireProject.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{
						releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1"),
						releases.NewRelease(defaultChannel.ID, fireProjectID, "2.0"),
						releases.NewRelease(betaChannel.ID, fireProjectID, "2.0-beta2"),
						releases.NewRelease(betaChannel.ID, fireProjectID, "2.0-beta1"),
					},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1%2CChannels-13&take=2").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel, betaChannel},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				VERSION    CHANNEL
				2.1        Default Channel
				2.0        Default Channel
				2.0-beta2  Beta Channel
				2.0-beta1  Beta Channel
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"release list picks up project from flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", "--project", fireProject.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1")},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1&take=1").
				RespondWith(resources.Resources[*channels.Channel]{Items: []*channels.Channel{defaultChannel}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				VERSION  CHANNEL
				2.1      Default Channel
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"release list picks up project from short flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", "-p", fireProject.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1")},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1&take=1").
				RespondWith(resources.Resources[*channels.Channel]{Items: []*channels.Channel{defaultChannel}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				VERSION  CHANNEL
				2.1      Default Channel
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", "-p", fireProject.Name, "--output-format", "json", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1")},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1&take=1").
				RespondWith(resources.Resources[*channels.Channel]{Items: []*channels.Channel{defaultChannel}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				Channel   string
				ChannelID string
				Version   string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{{
				Channel:   defaultChannel.Name,
				ChannelID: defaultChannel.ID,
				Version:   "2.1",
			}}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},
		{"outputFormat basic", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "list", "-p", fireProject.Name, "--output-format", "basic", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{releases.NewRelease(defaultChannel.ID, fireProjectID, "2.1")},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels?ids=Channels-1&take=1").
				RespondWith(resources.Resources[*channels.Channel]{Items: []*channels.Channel{defaultChannel}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				2.1
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
