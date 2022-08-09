package fixtures

import (
	"bytes"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
)

// NewCobraRootCommand creates the Cobra root command, and captures its stdout/stderr into buffers so you can
// assert on them later
func NewCobraRootCommand(fac *testutil.MockFactory) (cmd *cobra.Command, stdout *bytes.Buffer, stderr *bytes.Buffer) {
	cmd = cmdRoot.NewCmdRoot(fac, nil, nil)

	var bufferedOut bytes.Buffer
	cmd.SetOut(&bufferedOut)
	stdout = &bufferedOut

	var bufferedErr bytes.Buffer
	cmd.SetErr(&bufferedErr)
	stderr = &bufferedErr

	return
}
