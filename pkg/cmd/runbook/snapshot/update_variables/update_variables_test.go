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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestRunbookSnapshotUpdateVariables(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"
	const waterProjectID = "Projects-23"
	const runbookID = "Runbooks-1"
	const otherRunbookID = "Runbooks-2"
	const publishedSnapshotID = "RunbookSnapshots-1"
	const otherSnapshotID = "RunbookSnapshots-2"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	waterProject := fixtures.NewProject(spaceID, waterProjectID, "Water Project", "Lifecycles-1", "ProjectGroups-1", "")
	rebuildIndexes := fixtures.NewRunbook(spaceID, fireProjectID, runbookID, "Rebuild DB Indexes")
	rebuildIndexes.PublishedRunbookSnapshotID = publishedSnapshotID
	healthCheck := fixtures.NewRunbook(spaceID, fireProjectID, otherRunbookID, "Health Check")

	publishedSnapshot := fixtures.NewRunbookSnapshot(fireProjectID, runbookID, publishedSnapshotID, "Snapshot ABC123")
	otherSnapshot := fixtures.NewRunbookSnapshot(fireProjectID, runbookID, otherSnapshotID, "Snapshot 40C9ENM")

	expectProjectAndRunbookLookup := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
		// FindRunbook tries GetByID first, falls back to GetByName
		api.ExpectRequest(t, "GET", "/api/Spaces-1/runbooks/Rebuild DB Indexes").
			RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?partialName=Rebuild+DB+Indexes").
			RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{rebuildIndexes},
			})
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"noprompt: missing --project returns clear error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables", "--runbook", rebuildIndexes.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")
		}},

		{"noprompt: missing --runbook returns clear error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables", "--project", fireProject.Name, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "runbook must be specified")
		}},

		{"noprompt with --snapshot: posts to named snapshot", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables",
					"--project", fireProject.Name,
					"--runbook", rebuildIndexes.Name,
					"--snapshot", otherSnapshot.Name,
					"--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectAndRunbookLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbookSnapshots/Snapshot 40C9ENM").
				RespondWith(otherSnapshot)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbookSnapshots/RunbookSnapshots-2/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot 'Snapshot 40C9ENM'")
			assert.Contains(t, stdOut.String(), "for runbook 'Rebuild DB Indexes'")
			// when --snapshot is provided, no "Updating variables for published snapshot" notice
			assert.NotContains(t, stdOut.String(), "Updating variables for published snapshot")
		}},

		{"noprompt without --snapshot: defaults to published snapshot and announces it", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables",
					"--project", fireProject.Name,
					"--runbook", rebuildIndexes.Name,
					"--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectAndRunbookLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbookSnapshots/RunbookSnapshots-1").
				RespondWith(publishedSnapshot)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbookSnapshots/RunbookSnapshots-1/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Updating variables for published snapshot 'Snapshot ABC123'")
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot 'Snapshot ABC123'")
			assert.Contains(t, stdOut.String(), "for runbook 'Rebuild DB Indexes'")
		}},

		{"runbook with no published snapshot and no --snapshot returns guidance error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			noPublished := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-99", "Restart App")

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables",
					"--project", fireProject.Name,
					"--runbook", noPublished.Name,
					"--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{Items: []*projects.Project{fireProject}})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/runbooks/Restart App").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?partialName=Restart+App").
				RespondWith(resources.Resources[*runbooks.Runbook]{Items: []*runbooks.Runbook{noPublished}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "runbook has no published snapshot; specify a snapshot with --snapshot")
		}},

		{"server returns non-2xx status returns wrapped error including body", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables",
					"--project", fireProject.Name,
					"--runbook", rebuildIndexes.Name,
					"--no-prompt"})
				return rootCmd.ExecuteC()
			})

			expectProjectAndRunbookLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbookSnapshots/RunbookSnapshots-1").
				RespondWith(publishedSnapshot)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbookSnapshots/RunbookSnapshots-1/snapshot-variables").
				RespondWithStatus(409, "409 Conflict", "snapshot is locked")

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update variable snapshot (HTTP 409)")
			assert.Contains(t, err.Error(), "snapshot is locked")
		}},

		{"interactive: prompts for project and runbook then updates published snapshot with automation command", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "snapshot", "update-variables"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").
				RespondWith([]*projects.Project{fireProject, waterProject})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project containing the runbook:",
				Options: []string{fireProject.Name, waterProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{Items: []*runbooks.Runbook{rebuildIndexes, healthCheck}})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the runbook:",
				Options: []string{rebuildIndexes.Name, healthCheck.Name},
			}).AnswerWith(rebuildIndexes.Name)

			// after prompts, run path re-resolves project + runbook and resolves the published snapshot
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{Items: []*projects.Project{fireProject}})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/runbooks/Rebuild DB Indexes").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?partialName=Rebuild+DB+Indexes").
				RespondWith(resources.Resources[*runbooks.Runbook]{Items: []*runbooks.Runbook{rebuildIndexes}})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbookSnapshots/RunbookSnapshots-1").
				RespondWith(publishedSnapshot)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbookSnapshots/RunbookSnapshots-1/snapshot-variables").RespondWith(nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Updating variables for published snapshot 'Snapshot ABC123'")
			assert.Contains(t, stdOut.String(), "Successfully updated variable snapshot 'Snapshot ABC123'")
			assert.Contains(t, stdOut.String(), "Automation Command:")
			assert.Contains(t, stdOut.String(), "--project 'Fire Project'")
			assert.Contains(t, stdOut.String(), "--runbook 'Rebuild DB Indexes'")
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
