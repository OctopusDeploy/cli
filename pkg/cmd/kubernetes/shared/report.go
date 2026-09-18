package shared

import (
	"context"
	"fmt"
	"io"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

func ReportInstalled(out io.Writer, release helm.Release) {
	fmt.Fprintf(out, "\n%s Installed %s %s as release %s in namespace %s.\n",
		output.Green("✔"), release.Chart, release.Version,
		output.Cyan(release.Name), output.Cyan(release.Namespace))
}

// PrintAutomationCommand shows how to reproduce the run without the wizard, so
// there is nothing to print when there was no wizard.
func PrintAutomationCommand(d *cmd.Dependencies, generatable []flag.Generatable) {
	if d.NoPrompt {
		return
	}
	autoCmd := flag.GenerateAutomationCmd(d.CmdPath, d.GetSpaceNameOrEmpty(), generatable...)
	fmt.Fprintf(d.Out, "\nAutomation Command: %s\n", autoCmd)
}

// RenderOnly backs --dry-run. detail spells out what this component skips when
// nothing is installed; a nil preflight means it has no connectivity targets.
func RenderOnly(ctx context.Context, d *cmd.Dependencies, runner *helm.Runner, spec helm.InstallSpec, detail string, preflight *Preflight) error {
	fmt.Fprintf(d.Out, "\n%s Rendering only. %s\n", output.Dim("--"+octoK8s.FlagDryRun), detail)

	if preflight != nil {
		preflight.ReportStatic()
	}

	manifest, err := runner.Render(ctx, spec)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.Out, manifest)
	return nil
}
