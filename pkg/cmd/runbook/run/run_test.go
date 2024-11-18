package run_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

var now = func() time.Time {
	return time.Date(2022, time.September, 8, 13, 25, 2, 0, time.FixedZone("Malaysia", 8*3600)) // UTC+8
}
var ctxWithFakeNow = context.WithValue(context.TODO(), constants.ContextKeyTimeNow, now)

// These tests ensure that given the right input, we call the server's API appropriately
// they all run in automation mode where survey is disabled; they'd error if they tried to ask questions
func TestRunbookRun_AutomationMode(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)
	_ = fireProject

	// TEST STARTS HERE
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"runbook run requires a project name", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run requires a runbook name", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "runbook name must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run requires at least one environment", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "environment(s) must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run specifying project, runbook, env only (bare minimum) assuming untenanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database", "--environment", "dev"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1")
			requestBody, err := testutil.ReadJson[runbooks.RunbookRunCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, runbooks.RunbookRunCommandV1{
				RunbookName:      "Provision Database",
				EnvironmentNames: []string{"dev"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: []*runbooks.RunbookRunServerTask{
					{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
					{RunbookRunID: "RunbookRun-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "Successfully started 2 runbook run(s)\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run specifying project, runbook, env only (bare minimum) assuming untenanted; basic output format", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database", "--environment", "dev", "--output-format", constants.OutputFormatBasic})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1").RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: []*runbooks.RunbookRunServerTask{
					{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
					{RunbookRunID: "RunbookRun-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				ServerTasks-29394
				ServerTasks-55312
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run specifying project, runbook, env only (bare minimum) assuming untenanted; json output format", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database", "--environment", "dev", "--output-format", constants.OutputFormatJson})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			serverTasks := []*runbooks.RunbookRunServerTask{
				{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
				{RunbookRunID: "RunbookRun-204", ServerTaskID: "ServerTasks-55312"},
			}

			api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1").RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: serverTasks,
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			var response []*runbooks.RunbookRunServerTask
			err = json.Unmarshal(stdOut.Bytes(), &response)
			assert.Nil(t, err)

			assert.Equal(t, serverTasks, response)

			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run specifying project, runbook, env only (bare minimum) assuming tenanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database", "--environment", "dev", "--tenant", "Coke", "--tenant", "Pepsi"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1")
			requestBody, err := testutil.ReadJson[runbooks.RunbookRunCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, runbooks.RunbookRunCommandV1{
				RunbookName:      "Provision Database",
				EnvironmentNames: []string{"dev"},
				Tenants:          []string{"Coke", "Pepsi"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: []*runbooks.RunbookRunServerTask{
					{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
					{RunbookRunID: "RunbookRun-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "Successfully started 2 runbook run(s)\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook run specifying project, runbook, env only (bare minimum) assuming tenanted via tags", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "run", "--project", "Fire Project", "--runbook", "Provision Database", "--environment", "dev", "--tenant-tag", "Regions/us-west", "--tenant-tag", "Importance/High"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1")
			requestBody, err := testutil.ReadJson[runbooks.RunbookRunCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, runbooks.RunbookRunCommandV1{
				RunbookName:      "Provision Database",
				EnvironmentNames: []string{"dev"},
				TenantTags:       []string{"Regions/us-west", "Importance/High"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: []*runbooks.RunbookRunServerTask{
					{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
					{RunbookRunID: "RunbookRun-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "Successfully started 2 runbook run(s)\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying all the args", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{
					"runbook", "run",
					"--project", "Fire Project",
					"--runbook", "Provision Database",
					"--environment", "dev", "--environment", "test",
					"--run-at", "2022-09-10 13:32:03 +10:00",
					"--run-at-expiry", "2022-09-10 13:37:03 +10:00",
					"--skip", "Install", "--skip", "Cleanup",
					"--snapshot", "Snapshot FWKMLUX",
					"--guided-failure", "true",
					"--force-package-download",
					"--target", "firstMachine", "--target", "secondMachine",
					"--exclude-target", "thirdMachine",
					"--variable", "Approver:John", "--variable", "Signoff:Jane",
					"--output-format", "basic",
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/runbook-runs/create/v1")
			requestBody, err := testutil.ReadJson[runbooks.RunbookRunCommandV1](req.Request.Body)
			assert.Nil(t, err)

			trueVar := true
			assert.Equal(t, runbooks.RunbookRunCommandV1{
				RunbookName:      "Provision Database",
				EnvironmentNames: []string{"dev", "test"},
				Snapshot:         "Snapshot FWKMLUX",
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:              "Spaces-1",
					ProjectIDOrName:      fireProject.Name,
					ForcePackageDownload: true,
					SpecificMachineNames: []string{"firstMachine", "secondMachine"},
					ExcludedMachineNames: []string{"thirdMachine"},
					SkipStepNames:        []string{"Install", "Cleanup"},
					UseGuidedFailure:     &trueVar,
					RunAt:                "2022-09-10 13:32:03 +10:00",
					NoRunAfter:           "2022-09-10 13:37:03 +10:00",
					Variables: map[string]string{
						"Approver": "John",
						"Signoff":  "Jane",
					},
				},
			}, requestBody)

			req.RespondWith(&runbooks.RunbookRunResponseV1{
				RunbookRunServerTasks: []*runbooks.RunbookRunServerTask{
					{RunbookRunID: "RunbookRun-203", ServerTaskID: "ServerTasks-29394"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Contains(t, stdOut.String(), "ServerTasks-29394\n")
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
			rootCmd.SetContext(ctxWithFakeNow)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}

func TestRunbookRun_PrintAdvancedSummary(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, stdout *bytes.Buffer)
	}{
		{"default state", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
				Additional Options:
				  Run At: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Run Targets: All included
				`), stdout.String())
		}},

		{"all the things different", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ScheduledStartTime:   "2022-09-23",
				GuidedFailureMode:    "false",
				ForcePackageDownload: true,
				ExcludedSteps:        []string{"Step 1", "Step 37"},
				RunTargets:           []string{"vm-1", "vm-2"},
				ExcludeTargets:       []string{"vm-3", "vm-4"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
				Additional Options:
				  Run At: 2022-09-23
				  Skipped Steps: Step 1,Step 37
				  Guided Failure Mode: Do not use guided failure mode
				  Package Download: Re-download packages from feed
				  Run Targets: Include vm-1,vm-2; Exclude vm-3,vm-4
				`), stdout.String())
		}},

		{"variation on include deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				RunTargets: []string{"vm-2"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
				Additional Options:
				  Run At: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Run Targets: Include vm-2
				`), stdout.String())
		}},

		{"variation on exclude deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ExcludeTargets: []string{"vm-4"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
				Additional Options:
				  Run At: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Run Targets: Exclude vm-4
				`), stdout.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.run(t, new(bytes.Buffer))
		})
	}
}
