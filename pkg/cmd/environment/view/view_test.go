package view_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestEnvironmentView(t *testing.T) {
	const spaceID = "Spaces-1"
	const envID = "Environments-3"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	newDevEnvironment := func() *environments.Environment {
		env := fixtures.NewEnvironment(spaceID, envID, "Development")
		env.Slug = "development"
		env.UseGuidedFailure = false
		// opposite of UseGuidedFailure so the two columns can't be confused
		env.AllowDynamicInfrastructure = true
		return env
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"view by name renders each flag from its own field", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			env := newDevEnvironment()

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"environment", "view", "Development", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// not an ID, so the ID lookup 404s and the name lookup follows
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/Development").
				RespondWithStatus(http.StatusNotFound, "404 Not Found", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=Development").
				RespondWith(resources.Resources[*environments.Environment]{Items: []*environments.Environment{env}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME         SLUG         DESCRIPTION              GUIDED FAILURE  DYNAMIC INFRASTRUCTURE  WEB URL
				Development  development  No description provided  Disabled        Allowed                 http://server/app#/Spaces-1/infrastructure/environments/Environments-3
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"view by ID resolves via the ID lookup", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			env := newDevEnvironment()

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"environment", "view", envID, "--no-prompt", "-f", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// resolves on the first hop; no name lookup should follow
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/Environments-3").RespondWith(env)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Development (Environments-3)
				No description provided
				View this environment in Octopus Deploy: http://server/app#/Spaces-1/infrastructure/environments/Environments-3

				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"view returns an error when nothing matches", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"environment", "view", "NoSuchEnvironment", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/NoSuchEnvironment").
				RespondWithStatus(http.StatusNotFound, "404 Not Found", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments?partialName=NoSuchEnvironment").
				RespondWith(resources.Resources[*environments.Environment]{Items: []*environments.Environment{}})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "cannot find an environment with name or ID of 'NoSuchEnvironment'")

			assert.Equal(t, "", stdOut.String())
		}},

		{"view surfaces a non-404 failure rather than reporting not-found", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"environment", "view", "Development", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/Development").
				RespondWithStatus(http.StatusForbidden, "403 Forbidden", core.APIError{ErrorMessage: "You do not have permission"})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "You do not have permission")

			assert.Equal(t, "", stdOut.String())
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
			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}
