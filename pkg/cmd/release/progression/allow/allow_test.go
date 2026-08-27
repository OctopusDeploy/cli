package allow_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/defects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestReleaseProgressionAllow(t *testing.T) {
	const spaceID = "Spaces-1"
	const projectID = "Projects-22"
	const releaseID = "Releases-9"

	space1 := fixtures.NewSpace(spaceID, "Default Space")
	fireProject := fixtures.NewProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	release := releases.NewRelease("Channels-1", projectID, "1.0")
	release.ID = releaseID
	release.SpaceID = spaceID

	blockedDefect := &defects.Defect{Status: defects.DefectStatusUnresolved}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		// Regression: --project supplied interactively used to leave selectedProject nil,
		// which panicked on selectedProject.GetName().
		{"allow prompts for the release when the project is supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "progression", "allow", "--project", projectID})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// the supplied project is looked up so the release prompt has something to work with
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases").
				RespondWith(resources.Resources[*releases.Release]{Items: []*releases.Release{release}})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Release to Allow Progression for",
				Options: []string{"1.0"},
			}).AnswerWith("1.0")

			// PromptMissing normalises Project.Value to the project name, so the lookup is by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/1.0").RespondWith(release)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-9/defects").
				RespondWith(resources.Resources[*defects.Defect]{Items: []*defects.Defect{blockedDefect}})
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-9/defects/resolve").
				RespondWith(&defects.Defect{Status: defects.DefectStatusResolved})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully allowed progression for release 1.0")
			assert.Equal(t, "", stdErr.String())
		}},

		// Regression: --version supplied interactively used to leave selectedRelease nil,
		// which panicked on selectedRelease.Version.
		{"allow prompts for the project when the version is supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "progression", "allow", "--version", "1.0"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project in which the blocked release exists",
				Options: []string{"Fire Project"},
			}).AnswerWith("Fire Project")

			// --version was supplied, so no release prompt is shown; we go straight to the lookup
			// PromptMissing normalises Project.Value to the project name, so the lookup is by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/releases/1.0").RespondWith(release)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-9/defects").
				RespondWith(resources.Resources[*defects.Defect]{Items: []*defects.Defect{blockedDefect}})
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/Releases-9/defects/resolve").
				RespondWith(&defects.Defect{Status: defects.DefectStatusResolved})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully allowed progression for release 1.0")
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
