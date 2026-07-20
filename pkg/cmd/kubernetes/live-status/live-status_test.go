package livestatus_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

const spaceID = "Spaces-1"

func respondToSpaceScopedInit(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
}

func TestKubernetesLiveStatus(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")
	fireProject := fixtures.NewProject(spaceID, "Projects-22", "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	liveStatusResponse := map[string]any{
		"MachineStatuses": []any{
			map[string]any{
				"MachineId": "Machines-1",
				"Status":    "Healthy",
				"Resources": []any{
					map[string]any{
						"Name":             "my-deployment",
						"Namespace":        "default",
						"Kind":             "Deployment",
						"Group":            "apps",
						"HealthStatus":     "Healthy",
						"SyncStatus":       "InSync",
						"ResourceSourceId": "Machines-1",
						"SourceType":       "KubernetesMonitor",
						"Children": []any{
							map[string]any{
								"Name":             "my-deployment-abc123",
								"Namespace":        "default",
								"Kind":             "ReplicaSet",
								"Group":            "apps",
								"HealthStatus":     "Healthy",
								"SyncStatus":       "InSync",
								"ResourceSourceId": "Machines-1",
								"SourceType":       "KubernetesMonitor",
								"Children":         []any{},
								"LastUpdated":      "2026-01-15T10:30:00Z",
							},
						},
						"LastUpdated": "2026-01-15T10:30:00Z",
					},
				},
			},
		},
		"Summary": map[string]any{
			"Status":       "Healthy",
			"HealthStatus": "Healthy",
			"SyncStatus":   "InSync",
			"LastUpdated":  "2026-01-15T10:30:00Z",
		},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"requires project in automation mode", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--no-prompt", "--environment", "Production"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified; use --project flag or run in interactive mode")
		}},

		{"requires environment in automation mode", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--no-prompt", "--project", "Fire Project"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			// project lookup
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "environment must be specified; use --environment flag or run in interactive mode")
		}},

		{"makes untenanted request with correct URL", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--project", "Fire Project", "--environment", "Production", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			// project lookup
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)

			// environment lookup - returns empty results so FindEnvironment fails
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Production").
				RespondWith(map[string]any{
					"Items":        []any{},
					"ItemsPerPage": 30,
					"TotalResults": 0,
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Production")
		}},

		{"makes untenanted request and returns JSON output", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--project", "Fire Project", "--environment", "Production", "--no-prompt", "-f", "json"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			// project lookup by name -> found directly
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)

			// environment lookup
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Production").
				RespondWith(map[string]any{
					"Items": []any{
						map[string]any{
							"Id":   "Environments-1",
							"Name": "Production",
							"Links": map[string]string{
								"Self": "/api/Spaces-1/environments/Environments-1",
							},
						},
					},
					"ItemsPerPage": 30,
					"TotalResults": 1,
				})

			// live status API call
			req := api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/environments/Environments-1/untenanted/livestatus")
			req.RespondWith(liveStatusResponse)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			out := stdOut.String()
			assert.Contains(t, out, `"MachineStatuses"`)
			assert.Contains(t, out, `"my-deployment"`)
			assert.Contains(t, out, `"Healthy"`)
		}},

		{"table output shows machine as top-level node", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--project", "Fire Project", "--environment", "Production", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Production").
				RespondWith(map[string]any{
					"Items": []any{
						map[string]any{
							"Id":   "Environments-1",
							"Name": "Production",
							"Links": map[string]string{
								"Self": "/api/Spaces-1/environments/Environments-1",
							},
						},
					},
					"ItemsPerPage": 30,
					"TotalResults": 1,
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/environments/Environments-1/untenanted/livestatus").
				RespondWith(liveStatusResponse)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			out := stdOut.String()
			// Machine should appear as a top-level node
			assert.Contains(t, out, "Machines-1")
			assert.Contains(t, out, "Machine")
			// Resources should be indented under the machine
			assert.Contains(t, out, "  my-deployment")
			assert.Contains(t, out, "    my-deployment-abc123")
		}},

		{"summary-only adds query parameter", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"kubernetes", "live-status", "--project", "Fire Project", "--environment", "Production", "--summary-only", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWith(fireProject)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Production").
				RespondWith(map[string]any{
					"Items": []any{
						map[string]any{
							"Id":   "Environments-1",
							"Name": "Production",
							"Links": map[string]string{
								"Self": "/api/Spaces-1/environments/Environments-1",
							},
						},
					},
					"ItemsPerPage": 30,
					"TotalResults": 1,
				})

			r, _ := api.ReceiveRequest()
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.String(), "/livestatus")
			assert.Equal(t, "true", r.URL.Query().Get("summaryOnly"))

			responseBytes, _ := json.Marshal(liveStatusResponse)
			api.Respond(&http.Response{
				StatusCode:    200,
				Status:        "200 OK",
				Body:          io.NopCloser(bytes.NewReader(responseBytes)),
				ContentLength: int64(len(responseBytes)),
				Header:        make(http.Header),
			}, nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			out := stdOut.String()
			assert.Contains(t, out, "Healthy")
			assert.Contains(t, out, "InSync")
		}},

		{"k8s alias works", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"k8s", "live-status", "--no-prompt", "--environment", "Production"})
				return rootCmd.ExecuteC()
			})

			respondToSpaceScopedInit(t, api)

			_, err := testutil.ReceivePair(cmdReceiver)
			// Should get the same error as when using "kubernetes" - proves the alias routes correctly
			assert.EqualError(t, err, "project must be specified; use --project flag or run in interactive mode")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdOut, stdErr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdOut)
			rootCmd.SetErr(stdErr)
			test.run(t, api, rootCmd, stdOut, stdErr)
		})
	}
}
