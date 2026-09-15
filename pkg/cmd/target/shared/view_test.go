package shared_test

import (
	"bytes"
	"testing"

	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var viewRootResource = testutil.NewRootResource()

// the per-type views all render through shared.ViewRun, so one of them is enough
// to cover the Disabled row it adds
func TestPerTypeViewShowsDisabledState(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")
	development := fixtures.NewEnvironment(spaceID, "Environments-1", "Development")

	for _, tc := range []struct {
		name       string
		isDisabled bool
		expected   string
	}{
		{"disabled target", true, "true"},
		{"enabled target", false, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"deployment-target", "cloud-region", "view", "Machines-100", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(viewRootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(viewRootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/machines/Machines-100").
				RespondWith(fixtures.NewDeploymentTarget(spaceID, "Machines-100", "web-server", tc.isDisabled))
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{development})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Regexp(t, `Disabled\s+`+tc.expected, stdout.String())
			assert.Equal(t, "", stderr.String())
		})
	}
}
