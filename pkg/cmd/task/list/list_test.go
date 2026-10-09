package list_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = newRootResource()

// newRootResource extends the shared test root resource with the tasks link, which
// the unit test server does not provide. Real servers expose more query parameters
// than this minimal template.
func newRootResource() *client.RootResource {
	root := testutil.NewRootResource()
	root.Links[constants.LinkTasks] = "/api/Spaces-1/tasks{/id}{?environment,ids,project,skip,states,take}"
	return root
}

func TestTaskList(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")
	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "Development")
	prEnvironment := fixtures.NewEphemeralEnvironment(spaceID, "Environments-99", "123-pr", "Environments-1")

	started := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	completed := started.Add(95 * time.Second)

	executingTask := tasks.NewTask()
	executingTask.ID = "ServerTasks-1"
	executingTask.Description = "Deploy Fire Project release 1.2.3 to Development"
	executingTask.State = "Executing"
	executingTask.StartTime = &started

	successTask := tasks.NewTask()
	successTask.ID = "ServerTasks-2"
	successTask.Description = "Deploy Fire Project release 1.2.2 to Development"
	successTask.State = "Success"
	successTask.StartTime = &started
	successTask.CompletedTime = &completed

	taskPage := func(items ...*tasks.Task) resources.Resources[*tasks.Task] {
		return resources.Resources[*tasks.Task]{Items: items}
	}

	expectProjectLookup := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
	}

	expectSpaceLookup := func(t *testing.T, api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"no filters lists tasks with the default limit", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=30").RespondWith(taskPage(executingTask, successTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "ID")
			assert.Contains(t, stdOut.String(), "ServerTasks-1")
			assert.Contains(t, stdOut.String(), "Deploy Fire Project release 1.2.3 to Development")
			assert.Contains(t, stdOut.String(), "ServerTasks-2")
			assert.Contains(t, stdOut.String(), "1m35s")
			assert.Equal(t, "", stdErr.String())
		}},

		{"ls alias works", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "ls"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=30").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "ServerTasks-1")
		}},

		{"basic output prints only task IDs", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=30").RespondWith(taskPage(executingTask, successTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\nServerTasks-2\n", stdOut.String())
		}},

		{"json output includes task details", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--output-format", "json"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=30").RespondWith(taskPage(executingTask, successTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			var parsed []map[string]any
			assert.Nil(t, json.Unmarshal(stdOut.Bytes(), &parsed))
			assert.Len(t, parsed, 2)
			assert.Equal(t, "ServerTasks-1", parsed[0]["ID"])
			assert.Equal(t, "Executing", parsed[0]["State"])
			assert.Equal(t, "ServerTasks-2", parsed[1]["ID"])
			assert.Equal(t, "Success", parsed[1]["State"])
			assert.Equal(t, "1m35s", parsed[1]["Duration"])
		}},

		{"no matching tasks prints nothing in basic output", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=30").RespondWith(taskPage())

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "", stdOut.String())
		}},

		{"--project resolves the project and filters by its ID", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--project", "Fire Project", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			expectProjectLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?project=Projects-22&take=30").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"--state accepts repeated and comma separated values in any case", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--state", "queued,EXECUTING", "--state", "cancelling", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?states=Queued%2CExecuting%2CCancelling&take=30").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"unknown --state returns a clear error before calling the server", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--state", "Running"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "unknown task state 'Running'")
		}},

		{"negative --limit returns a clear error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--limit", "-1"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "--limit must be zero or greater")
		}},

		{"--limit is passed to the server as take", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--limit", "5", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?take=5").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"--limit 0 follows the paging links to list every matching task", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--limit", "0", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			firstPage := taskPage(executingTask)
			firstPage.PagedResults.Links = resources.Links{PageNext: "/api/Spaces-1/tasks?skip=1&take=1"}

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks").RespondWith(firstPage)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?skip=1&take=1").RespondWith(taskPage(successTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\nServerTasks-2\n", stdOut.String())
		}},

		{"--environment given as an ID is used as is", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "Environments-12", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?environment=Environments-12&take=30").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"--environment given as a name resolves a regular environment", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "Development", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Development").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?environment=Environments-12&take=30").RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"--environment falls back to ephemeral environments", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "123-pr", "--state", "Queued,Executing,Cancelling", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=123-pr").
				RespondWith(resources.Resources[*environments.Environment]{})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/v2?skip=0&take=2147483647&partialName=123-pr&type=Ephemeral").
				RespondWith(resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{
					Items:        []*ephemeralenvironments.EphemeralEnvironment{prEnvironment},
					PagedResults: resources.PagedResults{TotalResults: 1},
				})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tasks?environment=Environments-99&states=Queued%2CExecuting%2CCancelling&take=30").
				RespondWith(taskPage(executingTask))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "ServerTasks-1\n", stdOut.String())
		}},

		{"--environment that matches nothing returns a clear error", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "Nowhere"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Nowhere").
				RespondWith(resources.Resources[*environments.Environment]{})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/v2?skip=0&take=2147483647&partialName=Nowhere&type=Ephemeral").
				RespondWith(resources.Resources[*ephemeralenvironments.EphemeralEnvironment]{})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "no environment or ephemeral environment found with name of Nowhere")
		}},

		{"--environment returns the real error when the environment lookup fails", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "Development"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			// no ephemeral lookup is expected: a failed lookup must not be treated as "not found"
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Development").
				RespondWithStatus(500, "InternalServerError", nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to look up environment 'Development'")
			assert.NotContains(t, err.Error(), "no environment or ephemeral environment found")
		}},

		{"--environment returns the real error when the ephemeral environment lookup fails", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"task", "list", "--environment", "123-pr"})
				return rootCmd.ExecuteC()
			})

			expectSpaceLookup(t, api)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=123-pr").
				RespondWith(resources.Resources[*environments.Environment]{})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/v2?skip=0&take=2147483647&partialName=123-pr&type=Ephemeral").
				RespondWithStatus(500, "InternalServerError", core.APIError{ErrorMessage: "the server fell over"})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to look up ephemeral environment '123-pr'")
			assert.NotContains(t, err.Error(), "no environment or ephemeral environment found")
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
