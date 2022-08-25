package delete_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"testing"
)

var rootResource = testutil.NewRootResource()

func TestReleaseDelete(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"
	const waterProjectID = "Projects-29"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	waterProject := fixtures.NewProject(spaceID, waterProjectID, "Water Project", "Lifecycles-1", "ProjectGroups-1", "")
	defaultChannelID := "Channels-1"
	betaChannelID := "Channels-13"

	rDefault21 := fixtures.NewRelease(spaceID, "Releases-21", "2.1", fireProjectID, defaultChannelID)
	rDefault20 := fixtures.NewRelease(spaceID, "Releases-20", "2.0", fireProjectID, defaultChannelID)
	rBeta20b2 := fixtures.NewRelease(spaceID, "Releases-12", "2.0-beta2", fireProjectID, betaChannelID)
	rBeta20b1 := fixtures.NewRelease(spaceID, "Releases-11", "2.0-beta1", fireProjectID, betaChannelID)

	// we have a load of tests which are all the same except they vary the cmdline args; this is a common test body
	standardDeleteTestBody := func(api *testutil.MockHttpServer, releaseIDs ...string) {
		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
			RespondWith(resources.Resources[*releases.Release]{
				Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
			})

		// then loop(delete release x)
		for _, id := range releaseIDs {
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+id).RespondWith(nil)
		}
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"noprompt: requires a project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: requires at least one version", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", "--project", fireProject.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "at least one release version must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: picks up version and project from flags and deletes matching releases", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", "--project", fireProject.Name, "--version", "2.0", "--version", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			standardDeleteTestBody(api, rDefault21.ID, rDefault20.ID)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())

		}},

		{"noprompt: picks up version and project from args and deletes matching releases", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", fireProject.Name, "2.0", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			standardDeleteTestBody(api, rDefault21.ID, rDefault20.ID)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: picks up version and project from args and deletes matching releases", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", fireProject.Name, "2.0", "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			standardDeleteTestBody(api, rDefault21.ID, rDefault20.ID)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: picks up project from first arg and versions from subsequent", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", "--version", "2.0", fireProject.Name, "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			standardDeleteTestBody(api, rDefault21.ID, rDefault20.ID)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"noprompt: picks up version from first arg if project is specified using a flag", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				// super weird, but it's possible so make sure it works
				rootCmd.SetArgs([]string{"release", "delete", "2.0", "--project", fireProject.Name, "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			standardDeleteTestBody(api, rDefault21.ID, rDefault20.ID)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		// ----- failure modes ------

		{"noprompt: error when deleting 1 release and it fails", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", fireProject.Name, "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
				})

			someKindOfNetworkError := errors.New("some kind of network error")
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rDefault21.ID).
				RespondWithError(someKindOfNetworkError)

			// this error output is terrible but we can't fix it until the go API client wrapper vNext
			nastyErrorString := "failed to delete release 2.1: cannot get endpoint /api/Spaces-1/releases/Releases-21 from server. failure from http client Delete \"http://server/api/Spaces-1/releases/Releases-21\": some kind of network error"

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Equal(t, &multierror.Error{
				Errors: []error{errors.New(nastyErrorString)},
			}, err)

			assert.Equal(t, "Failed to delete 1 releases\n", stdOut.String())
			assert.Equal(t, nastyErrorString+"\n", stdErr.String())
		}},

		{"noprompt: error when deleting 1 release and it fails due to HTTP statuscode rather than network error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", fireProject.Name, "2.1", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
				})

			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rDefault21.ID).RespondWithStatus(400, "400 Bad Request", struct {
				Details string
			}{Details: "server doesn't like this"})

			// this error output is terrible but we can't fix it until the go API client wrapper vNext
			nastyErrorString := "failed to delete release 2.1: bad request from endpoint /api/Spaces-1/releases/Releases-21. response from server 400 Bad Request"

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Equal(t, &multierror.Error{
				Errors: []error{errors.New(nastyErrorString)},
			}, err)

			assert.Equal(t, "Failed to delete 1 releases\n", stdOut.String())
			assert.Equal(t, nastyErrorString+"\n", stdErr.String())
		}},

		{"noprompt: error when deleting 4 releases and two fail; it keeps going past errors", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", fireProject.Name, rDefault21.Version, rDefault20.Version, rBeta20b2.Version, rBeta20b1.Version, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
				})

			// first one passes
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rDefault21.ID).RespondWith(nil)

			// second one fails
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rDefault20.ID).
				RespondWithError(errors.New("network error on 2.0"))

			// third also fails
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rBeta20b2.ID).
				RespondWithError(errors.New("network error on 2.0-beta2"))

			// final works
			api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+rBeta20b1.ID).RespondWith(nil)

			// this error output is terrible but we can't fix it until the go API client wrapper vNext
			nastyErrorString := func(r *releases.Release) string {
				return fmt.Sprintf("failed to delete release %s: cannot get endpoint /api/Spaces-1/releases/%s from server. failure from http client Delete \"http://server/api/Spaces-1/releases/%s\": network error on %s",
					r.Version, r.ID, r.ID, r.Version)
			}

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Equal(t, &multierror.Error{
				Errors: []error{
					errors.New(nastyErrorString(rDefault20)),
					errors.New(nastyErrorString(rBeta20b2)),
				},
			}, err)

			assert.Equal(t, "Deleted 2 releases. 2 releases failed\n", stdOut.String())
			assert.Equal(t, nastyErrorString(rDefault20)+"\n"+nastyErrorString(rBeta20b2)+"\n", stdErr.String())
		}},

		// ----- interactive ------

		{"interactive: prompt for everything and delete multiple releases with confirm", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").
				RespondWith([]*projects.Project{fireProject, waterProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to delete a release in",
				Options: []string{fireProject.Name, waterProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
				})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select Releases to delete",
				Options: []string{
					rDefault21.Version,
					rDefault20.Version,
					rBeta20b2.Version,
					rBeta20b1.Version,
				},
			}).AnswerWith([]string{rDefault21.Version, rDefault20.Version})

			q := qa.ExpectQuestion(t, &survey.Confirm{Message: "Confirm delete of 2 release(s)"})
			assert.Equal(t, heredoc.Doc(`
				You are about to delete the following releases:
				2.1
				2.0
				`), stdOut.String())
			stdOut.Reset()
			_ = q.AnswerWith(true)

			for _, id := range []string{rDefault21.ID, rDefault20.ID} {
				api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+id).RespondWith(nil)
			}

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 2 releases\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"interactive: project and releases specified on cmdline, only prompt for confirmation", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "delete", "Fire Project", "2.1"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{
					Items: []*releases.Release{rDefault21, rDefault20, rBeta20b2, rBeta20b1},
				})

			q := qa.ExpectQuestion(t, &survey.Confirm{Message: "Confirm delete of 1 release(s)"})
			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				You are about to delete the following releases:
				2.1
				`), stdOut.String())
			stdOut.Reset()
			_ = q.AnswerWith(true)

			for _, id := range []string{rDefault21.ID} {
				api.ExpectRequest(t, "DELETE", "/api/Spaces-1/releases/"+id).RespondWith(nil)
			}

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "Successfully deleted 1 releases\n", stdOut.String())
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
