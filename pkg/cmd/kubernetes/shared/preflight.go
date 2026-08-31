package shared

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/output"
)

// Preflight proves the endpoints a component needs are reachable from inside
// the cluster, which is what separates an install that works from one that
// succeeds and then never connects.
type Preflight struct {
	*cmd.Dependencies
	*octoK8s.CommonFlags

	Cluster   *octoK8s.Cluster
	Namespace string
	Targets   []octoK8s.Target
	// ProceedHelp says what is likely to go wrong if the install continues past
	// a failed check.
	ProceedHelp string
}

// Run reports the checks and decides whether the install goes ahead.
func (p *Preflight) Run(ctx context.Context) error {
	if p.SkipPreflight.Value || len(p.Targets) == 0 {
		return nil
	}

	checks := octoK8s.StaticChecks(p.Targets)
	podChecks, err := p.Cluster.RunPreflight(ctx, octoK8s.PreflightRequest{
		Namespace: p.Namespace,
		Image:     p.PreflightImage.Value,
		Targets:   p.Targets,
	})
	if err != nil {
		return err
	}
	checks = append(checks, podChecks...)

	return p.confirm(checks)
}

// ReportStatic covers a dry run, where there is no install to abandon and no
// namespace to put a check pod in.
func (p *Preflight) ReportStatic() {
	if p.SkipPreflight.Value || len(p.Targets) == 0 {
		return
	}
	PrintChecks(p.Out, connectivityHeading, octoK8s.StaticChecks(p.Targets))
}

func (p *Preflight) confirm(checks []octoK8s.Check) error {
	failed := PrintChecks(p.Out, connectivityHeading, checks)
	if failed == 0 {
		return nil
	}

	if p.NoPrompt {
		return fmt.Errorf("%d connectivity %s failed; fix the problems above or pass --%s",
			failed, octoK8s.Pluralise("check", "checks", failed), octoK8s.FlagSkipPreflight)
	}

	// A check can be wrong: egress policy may allow the real workload's service
	// account but not a bare pod.
	proceed := false
	if err := p.Ask(&survey.Confirm{
		Message: "Continue with the install anyway?",
		Default: false,
		Help:    p.ProceedHelp,
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("install cancelled")
	}
	return nil
}

const connectivityHeading = "Connectivity checks"

// PrintChecks writes the results and returns how many failed.
func PrintChecks(out io.Writer, heading string, checks []octoK8s.Check) int {
	if len(checks) == 0 {
		return 0
	}

	fmt.Fprintf(out, "\n%s:\n", heading)
	failed := 0
	for _, c := range checks {
		switch c.Result {
		case octoK8s.CheckPassed:
			fmt.Fprintf(out, "  %s %s %s\n", output.Green("✔"), c.Name, output.Dim(c.Detail))
		case octoK8s.CheckSkipped:
			fmt.Fprintf(out, "  %s %s %s\n", output.Dim("-"), c.Name, output.Dim(c.Detail))
		default:
			failed++
			fmt.Fprintf(out, "  %s %s %s\n", output.Red("✘"), c.Name, c.Detail)
			if c.Remediation != "" {
				fmt.Fprintf(out, "      %s\n", output.Dim(c.Remediation))
			}
		}
	}

	return failed
}

// CheckPermissions runs before anything is created, so a missing permission
// surfaces here rather than halfway through. A dry run creates nothing, so it
// reports the problem and carries on.
func CheckPermissions(ctx context.Context, d *cmd.Dependencies, cluster *octoK8s.Cluster, namespace string, dryRun bool) error {
	denied, err := cluster.CheckPermissions(ctx, octoK8s.InstallPermissions(namespace))
	if err != nil {
		return err
	}
	if len(denied) == 0 {
		return nil
	}

	message := fmt.Sprintf("your Kubernetes credentials cannot perform this install in context %q:", cluster.ContextName)
	for _, permission := range denied {
		message += fmt.Sprintf("\n  cannot %s - needed to %s", permission, permission.Description)
	}

	if dryRun {
		fmt.Fprintf(d.Out, "%s %s\n", output.Yellow("!"), message)
		return nil
	}
	return errors.New(message)
}

// EnsureNamespace creates the install namespace, so the credentials the chart
// needs can be written before Helm runs.
func EnsureNamespace(ctx context.Context, d *cmd.Dependencies, cluster *octoK8s.Cluster, namespace string) error {
	exists, err := cluster.NamespaceExists(ctx, namespace)
	if err != nil {
		return err
	}
	if exists {
		fmt.Fprintf(d.Out, "Using existing namespace %s\n", output.Cyan(namespace))
		return nil
	}
	return cluster.CreateNamespace(ctx, namespace)
}
