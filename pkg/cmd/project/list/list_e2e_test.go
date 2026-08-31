package list_test

import (
	"bytes"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

const spaceID = "Spaces-1"

func newProjectGroup(id string, name string) *projectgroups.ProjectGroup {
	group := projectgroups.NewProjectGroup(name)
	group.ID = id
	group.Links = map[string]string{
		"Projects": "/api/" + spaceID + "/projectgroups/" + id + "/projects",
		"Self":     "/api/" + spaceID + "/projectgroups/" + id,
	}
	return group
}

func TestProjectList(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, "Projects-1", "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	fireProject.Description = "the fire one"
	waterProject := fixtures.NewProject(spaceID, "Projects-2", "Water Project", "Lifecycles-1", "ProjectGroups-2", "")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"lists every project when no group is given", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").
				RespondWith([]*projects.Project{fireProject, waterProject})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
			NAME           DESCRIPTION   TAGS
			Fire Project   the fire one  
			Water Project                
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"lists only the group's projects when --group is given", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--group", "ProjectGroups-1"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1").
				RespondWith(newProjectGroup("ProjectGroups-1", "Default Project Group"))

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1/projects").
				RespondWith(&resources.Resources[*projects.Project]{Items: []*projects.Project{fireProject}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
			NAME          DESCRIPTION   TAGS
			Fire Project  the fire one  
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"accepts the -g shorthand and a group name, in json", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "ls", "-g", "Default Project Group", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// a name isn't an ID, so the lookup by ID misses first
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/Default Project Group").
				RespondWithStatus(404, "404 Not Found", nil)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups?partialName=Default+Project+Group").
				RespondWith(&resources.Resources[*projectgroups.ProjectGroup]{
					Items: []*projectgroups.ProjectGroup{newProjectGroup("ProjectGroups-1", "Default Project Group")},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/ProjectGroups-1/projects").
				RespondWith(&resources.Resources[*projects.Project]{Items: []*projects.Project{fireProject}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type projectJson struct {
				Id          string
				Name        string
				Description string
			}
			parsed, err := testutil.ParseJsonStrict[[]projectJson](stdOut)
			assert.Nil(t, err)
			assert.Equal(t, []projectJson{
				{Id: "Projects-1", Name: "Fire Project", Description: "the fire one"},
			}, parsed)
			assert.Equal(t, "", stdErr.String())
		}},

		{"reports an unknown group by the name that was asked for", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "list", "--group", "Nope"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/Nope").
				RespondWithStatus(404, "404 Not Found", nil)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups?partialName=Nope").
				RespondWith(&resources.Resources[*projectgroups.ProjectGroup]{Items: []*projectgroups.ProjectGroup{}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "cannot find a project group with name or ID of 'Nope'")
			assert.Equal(t, "", stdOut.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, nil)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, nil)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}
