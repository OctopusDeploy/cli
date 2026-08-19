package enable_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

const spaceID = "Spaces-1"

func newTarget(id string, name string, isDisabled bool) *machines.DeploymentTarget {
	target := machines.NewDeploymentTarget(name, machines.NewCloudRegionEndpoint(), []string{"Environments-1"}, []string{"web"})
	target.ID = id
	target.SpaceID = spaceID
	target.IsDisabled = isDisabled
	return target
}

func TestDeploymentTargetEnable(t *testing.T) {
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"enables a target identified on the command line", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "enable", "Machines-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-100").RespondWith(newTarget("Machines-100", "web-server", true))

			updateRequest := api.ExpectRequest(t, "PUT", "/api/Spaces-1/machines/Machines-100")
			updated, err := testutil.ReadJson[machines.DeploymentTarget](updateRequest.Request.Body)
			assert.Nil(t, err)
			assert.False(t, updated.IsDisabled)
			updateRequest.RespondWith(newTarget("Machines-100", "web-server", false))

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully enabled deployment target 'web-server'")
			assert.Equal(t, "", stdErr.String())
		}},

		{"does not update a target which is already enabled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "enable", "Machines-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-100").RespondWith(newTarget("Machines-100", "web-server", false))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "is already enabled")
			assert.Equal(t, "", stdErr.String())
		}},

		{"prompts for the target when none was supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "enable"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/machines?take=2147483647").
				RespondWith(resources.Resources[*machines.DeploymentTarget]{Items: []*machines.DeploymentTarget{
					newTarget("Machines-100", "web-server", true),
					newTarget("Machines-200", "db-server", true),
				}})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the deployment target you wish to enable:",
				Options: []string{"web-server", "db-server"},
			}).AnswerWith("web-server")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-100").RespondWith(newTarget("Machines-100", "web-server", true))
			api.ExpectRequest(t, "PUT", "/api/Spaces-1/machines/Machines-100").RespondWith(newTarget("Machines-100", "web-server", false))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Contains(t, stdOut.String(), "Successfully enabled deployment target 'web-server'")
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
