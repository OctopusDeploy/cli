package list_test

import (
	"bytes"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestProjectList(t *testing.T) {
	const spaceID = "Spaces-1"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	lifecycle := lifecycles.NewLifecycle("Default Lifecycle")
	lifecycle.ID = "Lifecycles-1"

	projectGroup := projectgroups.NewProjectGroup("Default Project Group")
	projectGroup.ID = "ProjectGroups-1"

	fireProject := fixtures.NewProject(spaceID, "Projects-22", "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	fireProject.SpaceID = spaceID
	fireProject.Slug = "fire-project"
	fireProject.ProjectTags = []string{"team/red"}

	waterProject := fixtures.NewProject(spaceID, "Projects-23", "Water Project", "Lifecycles-99", "ProjectGroups-1", "")
	waterProject.SpaceID = spaceID
	waterProject.Slug = "water-project"
	waterProject.Description = "Wet things"
	waterProject.IsDisabled = true
	waterProject.ProjectTags = []string{"team/blue"}

	expectListRequests := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject, waterProject})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/all").RespondWith([]*lifecycles.Lifecycle{lifecycle})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/all").RespondWith([]*projectgroups.ProjectGroup{projectGroup})
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"project list resolves group and lifecycle names, and falls back to the ID", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			expectListRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           SLUG           PROJECT GROUP          LIFECYCLE          DESCRIPTION  TAGS
				Fire Project   fire-project   Default Project Group  Default Lifecycle               team/red
				Water Project  water-project  Default Project Group  Lifecycles-99      Wet things   team/blue
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"project list still works when the lookups fail", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/all").RespondWithStatus(403, "403 Forbidden", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/all").RespondWithStatus(403, "403 Forbidden", nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME          SLUG          PROJECT GROUP    LIFECYCLE     DESCRIPTION  TAGS
				Fire Project  fire-project  ProjectGroups-1  Lifecycles-1               team/red
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			expectListRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				Id                     string
				Name                   string
				Description            string
				ProjectTags            []string
				Slug                   string
				SpaceId                string
				ProjectGroupId         string
				ProjectGroupName       string
				LifecycleId            string
				LifecycleName          string
				IsDisabled             bool
				IsVersionControlled    bool
				TenantedDeploymentMode string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{
				{
					Id:                     "Projects-22",
					Name:                   "Fire Project",
					ProjectTags:            []string{"team/red"},
					Slug:                   "fire-project",
					SpaceId:                spaceID,
					ProjectGroupId:         "ProjectGroups-1",
					ProjectGroupName:       "Default Project Group",
					LifecycleId:            "Lifecycles-1",
					LifecycleName:          "Default Lifecycle",
					TenantedDeploymentMode: "Untenanted",
				},
				{
					Id:                     "Projects-23",
					Name:                   "Water Project",
					Description:            "Wet things",
					ProjectTags:            []string{"team/blue"},
					Slug:                   "water-project",
					SpaceId:                spaceID,
					ProjectGroupId:         "ProjectGroups-1",
					ProjectGroupName:       "Default Project Group",
					LifecycleId:            "Lifecycles-99",
					IsDisabled:             true,
					TenantedDeploymentMode: "Untenanted",
				},
			}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic still lists just names", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			expectListRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Fire Project
				Water Project
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
