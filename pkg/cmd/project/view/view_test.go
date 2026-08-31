package view_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestProjectView(t *testing.T) {
	const spaceID = "Spaces-1"
	const projectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	lifecycle := lifecycles.NewLifecycle("Default Lifecycle")
	lifecycle.ID = "Lifecycles-1"

	projectGroup := projectgroups.NewProjectGroup("Default Project Group")
	projectGroup.ID = "ProjectGroups-1"

	fireProject := fixtures.NewProject(spaceID, projectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-Projects-22")
	fireProject.SpaceID = spaceID
	fireProject.Slug = "fire-project"
	fireProject.Description = "Fire things"
	fireProject.ProjectTags = []string{"team/red"}
	fireProject.VariableSetID = "variableset-Projects-22"
	fireProject.IncludedLibraryVariableSets = []string{"LibraryVariableSets-1"}

	expectViewRequests := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1").RespondWith(projectGroup)
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"project view (table)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "view", "Projects-22", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			expectViewRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME          SLUG          PROJECT GROUP          LIFECYCLE          DESCRIPTION  VERSION CONTROL         TAGS      WEB URL
				Fire Project  fire-project  Default Project Group  Default Lifecycle  Fire things  Not version controlled  team/red  http://server/app#/Spaces-1/projects/Projects-22
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"project view (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "view", "Projects-22", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			expectViewRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Fire Project (fire-project)
				Project group: Default Project Group
				Lifecycle: Default Lifecycle
				Tenanted deployment mode: Untenanted
				Version control branch: Not version controlled
				Tags: team/red
				Fire things
				Project is enabled
				View this project in Octopus Deploy: http://server/app#/Spaces-1/projects/Projects-22

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"project view falls back to IDs when the lookups fail (basic)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "view", "Projects-22", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWithStatus(403, "403 Forbidden", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1").RespondWithStatus(403, "403 Forbidden", nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Fire Project (fire-project)
				Project group: ProjectGroups-1
				Lifecycle: Lifecycles-1
				Tenanted deployment mode: Untenanted
				Version control branch: Not version controlled
				Tags: team/red
				Fire things
				Project is enabled
				View this project in Octopus Deploy: http://server/app#/Spaces-1/projects/Projects-22

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"project view does not panic when a version controlled project has no git settings", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// IsVersionControlled and PersistenceSettings are independent on the wire;
			// the CLI must not assume the settings block is present, or Git-typed.
			raw := map[string]any{}
			encoded, err := json.Marshal(fireProject)
			assert.Nil(t, err)
			assert.Nil(t, json.Unmarshal(encoded, &raw))
			raw["IsVersionControlled"] = true
			delete(raw, "PersistenceSettings")

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "view", "Projects-22", "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22").RespondWith(raw)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/Lifecycles-1").RespondWith(lifecycle)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1").RespondWith(projectGroup)

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Contains(t, stdOut.String(), "Version control branch: \n")
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "view", "Projects-22", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			expectViewRequests(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				Id                              string
				Name                            string
				Slug                            string
				Description                     string
				IsVersionControlled             bool
				VersionControlBranch            string
				ProjectTags                     []string
				WebUrl                          string
				SpaceId                         string
				IsDisabled                      bool
				ProjectGroupId                  string
				ProjectGroupName                string
				LifecycleId                     string
				LifecycleName                   string
				TenantedDeploymentMode          string
				DeploymentProcessId             string
				VariableSetId                   string
				IncludedLibraryVariableSetIds   []string
				AutoCreateRelease               bool
				DefaultToSkipIfAlreadyInstalled bool
				DiscreteChannelRelease          bool
				VersioningStrategy              *projects.VersioningStrategy
				ProjectConnectivityPolicy       *core.ConnectivityPolicy
			}
			parsedStdout, err := testutil.ParseJsonStrict[x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, x{
				Id:                            projectID,
				Name:                          "Fire Project",
				Slug:                          "fire-project",
				Description:                   "Fire things",
				VersionControlBranch:          "Not version controlled",
				ProjectTags:                   []string{"team/red"},
				WebUrl:                        "http://server/app#/Spaces-1/projects/Projects-22",
				SpaceId:                       spaceID,
				ProjectGroupId:                "ProjectGroups-1",
				ProjectGroupName:              "Default Project Group",
				LifecycleId:                   "Lifecycles-1",
				LifecycleName:                 "Default Lifecycle",
				TenantedDeploymentMode:        "Untenanted",
				DeploymentProcessId:           "deploymentprocess-Projects-22",
				VariableSetId:                 "variableset-Projects-22",
				IncludedLibraryVariableSetIds: []string{"LibraryVariableSets-1"},
				VersioningStrategy: &projects.VersioningStrategy{
					Template: "#{Octopus.Version.LastMajor}.#{Octopus.Version.LastMinor}.#{Octopus.Version.NextPatch}",
				},
				ProjectConnectivityPolicy: &core.ConnectivityPolicy{},
			}, parsedStdout)
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
