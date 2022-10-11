package create_test

import (
	"bytes"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/create"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("https://serverurl")
var spinner = &testutil.FakeSpinner{}
var rootResource = testutil.NewRootResource()

func TestCreateProjectShouldPromptMissing(t *testing.T) {

	const spaceID = "Spaces-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	out := &bytes.Buffer{}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"standard process asking all standard questions",
			func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
				opts := &create.CreateOptions{
					CreateFlags: create.NewCreateFlags(),
				}

				errReceiver := testutil.GoBegin(func() error {
					defer testutil.Close(api, qa)
					octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, "API-ABC123", spaceID)
					opts.Ask = qa.AsAsker()
					opts.Client = octopus
					opts.Out = out
					return create.PromptMissing(opts)
				})

				api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

				_ = qa.ExpectQuestion(t, &survey.Input{
					Message: "Name",
					Help:    "A short, memorable, unique name for this project.",
				}).AnswerWith("TestProject")

				lcs := []*lifecycles.Lifecycle{
					createLifecycle("lifecycle 1", "Lifecycles-1"),
					createLifecycle("lifecycle 2", "Lifecycles-2"),
				}
				api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/all").RespondWith(lcs)

				_ = qa.ExpectQuestion(t, &survey.Select{
					Message: "You have not specified a Lifecycle for this project. Please select one:",
					Options: []string{lcs[0].Name, lcs[1].Name},
				}).AnswerWith(lcs[0].Name)

				groups := []*projectgroups.ProjectGroup{
					createProjectGroup("Default project group", "ProjectGroups-1"),
					createProjectGroup("second group", "ProjectGroups-2"),
				}
				api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/all").RespondWith(groups)

				_ = qa.ExpectQuestion(t, &survey.Select{
					Message: "You have not specified a Project group for this project. Please select one:",
					Options: []string{groups[0].Name, groups[1].Name},
				}).AnswerWith(groups[0].Name)

				err := <-errReceiver
				assert.Nil(t, err)
				assert.Equal(t, opts.Name.Value, "TestProject")
				assert.Equal(t, opts.Lifecycle.Value, lcs[0].Name)
				assert.Equal(t, opts.Group.Value, groups[0].Name)
			}},
		{
			"standard process not requiring to ask any questions",
			func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
				opts := &create.CreateOptions{
					CreateFlags: create.NewCreateFlags(),
				}

				errReceiver := testutil.GoBegin(func() error {
					defer testutil.Close(api, qa)
					octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, "API-ABC123", spaceID)
					opts.Ask = qa.AsAsker()
					opts.Client = octopus
					opts.Out = out
					return create.PromptMissing(opts)
				})

				api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

				_ = qa.ExpectQuestion(t, &survey.Input{
					Message: "Name",
					Help:    "A short, memorable, unique name for this project.",
				}).AnswerWith("TestProject")

				lc := createLifecycle("lifecycle 1", "Lifecycles-1")
				api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/all").RespondWith([]*lifecycles.Lifecycle{lc})

				projectGroup := createProjectGroup("Default project group", "ProjectGroups-1")
				api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/all").RespondWith([]*projectgroups.ProjectGroup{projectGroup})

				err := <-errReceiver
				assert.Nil(t, err)
				assert.Equal(t, opts.Name.Value, "TestProject")
				assert.Equal(t, opts.Lifecycle.Value, lc.Name)
				assert.Equal(t, opts.Group.Value, projectGroup.Name)
			}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, new(bytes.Buffer))
		})
	}
}

func TestCreateProject_AutomationMode(t *testing.T) {
	space := fixtures.NewSpace("Spaces-1", "Default Space")
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"project creation", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			lc := createLifecycle("lifecycle 1", "Lifecycles-1")
			projectGroup := createProjectGroup("Default project group", "ProjectGroups-1")

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"project", "create", "--name", "TestProject", "--lifecycle", lc.ID, "--group", projectGroup.ID})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/lifecycles/"+lc.ID).RespondWith(lc)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projectgroups/"+projectGroup.ID).RespondWith(projectGroup)
			api.ExpectRequest(t, "POST", "/api/Spaces-1/projects").RespondWithStatus(201, "CREATED", projects.NewProject("TestProject", lc.ID, projectGroup.ID))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space), nil, nil)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}

func createLifecycle(name string, id string) *lifecycles.Lifecycle {
	lc := lifecycles.NewLifecycle(name)
	lc.ID = id
	return lc
}

func createProjectGroup(name string, id string) *projectgroups.ProjectGroup {
	group := projectgroups.NewProjectGroup(name)
	group.ID = id
	return group
}
