package api_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// respondToSdkInit handles the two HTTP requests that the Octopus SDK makes
// when initialising the system client: fetching the root resource and listing
// spaces to find the default space.
func respondToSdkInit(t *testing.T, api *testutil.MockHttpServer) {
	api.ExpectRequest(t, "GET", "/api/").RespondWith(testutil.NewRootResource())
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(map[string]any{
		"Items":        []any{},
		"ItemsPerPage": 30,
		"TotalResults": 0,
	})
}

func TestApiCommand(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"prints pretty-printed JSON response", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"api", "/api"})
				return rootCmd.ExecuteC()
			})

			respondToSdkInit(t, api)

			api.ExpectRequest(t, "GET", "/api").RespondWithStatus(http.StatusOK, "200 OK", map[string]string{
				"Application": "Octopus Deploy",
				"Version":     "2024.1.0",
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), `"Application": "Octopus Deploy"`)
			assert.Contains(t, stdOut.String(), `"Version": "2024.1.0"`)
		}},

		{"prints error response body on non-2xx status", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"api", "/api/nonexistent"})
				return rootCmd.ExecuteC()
			})

			respondToSdkInit(t, api)

			api.ExpectRequest(t, "GET", "/api/nonexistent").RespondWithStatus(http.StatusNotFound, "404 Not Found", map[string]string{
				"ErrorMessage": "Not found",
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "Not found")
		}},

		{"outputs raw body when response is not valid JSON", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"api", "/api/health"})
				return rootCmd.ExecuteC()
			})

			respondToSdkInit(t, api)

			r, _ := api.ReceiveRequest()
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/health", r.URL.Path)
			api.Respond(&http.Response{
				StatusCode:    http.StatusOK,
				Status:        "200 OK",
				Body:          io.NopCloser(bytes.NewReader([]byte("OK"))),
				ContentLength: 2,
			}, nil)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, "OK", stdOut.String())
		}},

		{"rejects path not prefixed with /api", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			defer api.Close()
			rootCmd.SetArgs([]string{"api", "/some/other/path"})
			_, err := rootCmd.ExecuteC()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "only supports paths prefixed with /api")
		}},

		{"requires an argument", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			defer api.Close()
			rootCmd.SetArgs([]string{"api"})
			_, err := rootCmd.ExecuteC()
			assert.Error(t, err)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdOut, stdErr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()
			fac := testutil.NewMockFactory(api)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, nil)
			rootCmd.SetOut(stdOut)
			rootCmd.SetErr(stdErr)
			test.run(t, api, rootCmd, stdOut, stdErr)
		})
	}
}
