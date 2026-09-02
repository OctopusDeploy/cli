package install

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

func (opts *InstallOptions) Commit(ctx context.Context) error {
	timeout, err := opts.ResolveTimeout()
	if err != nil {
		return err
	}

	values := opts.BuildValues()

	if err := shared.WriteValuesFile(opts.Out, opts.OutputValues.Value, values, ""); err != nil {
		return err
	}

	if err := shared.CheckPermissions(ctx, opts.Dependencies, opts.Cluster, opts.TargetNamespace, opts.DryRun.Value); err != nil {
		return err
	}

	if err := opts.confirmPrerequisites(); err != nil {
		return err
	}

	if opts.DryRun.Value {
		return opts.renderOnly(ctx, values, timeout)
	}

	if err := shared.EnsureNamespace(ctx, opts.Dependencies, opts.Cluster, opts.TargetNamespace); err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "\nInstalling the permissions controller into %s...\n", output.Cyan(opts.TargetNamespace))

	release, err := opts.Runner.Install(ctx, helm.InstallSpec{
		Chart:       opts.chartRef(),
		ReleaseName: opts.TargetRelease,
		Namespace:   opts.TargetNamespace,
		Values:      values,
		Atomic:      opts.Atomic.Value,
		Wait:        opts.Wait.Value,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}

	opts.reportSuccess(release)
	return nil
}

// BuildValues sets only what was chosen, leaving everything else at the chart's
// own defaults so a later chart version is free to change them.
func (opts *InstallOptions) BuildValues() map[string]any {
	values := map[string]any{
		"certManager": map[string]any{"enable": opts.CertManager.Value},
		"rbac":        map[string]any{"namespaced": opts.NamespacedRBAC.Value},
	}

	// envOverrides is the chart's own hook for replacing a single variable, which
	// setting manager.env would not do - it is a list, so it replaces the lot.
	overrides := map[string]any{}
	if len(opts.TargetNamespaces.Value) > 0 {
		overrides["TARGET_NAMESPACES"] = strings.Join(opts.TargetNamespaces.Value, ",")
	}
	if opts.TargetNamespaceRegex.Value != "" {
		overrides["TARGET_NAMESPACE_REGEX"] = opts.TargetNamespaceRegex.Value
	}
	if len(overrides) > 0 {
		values["manager"] = map[string]any{"envOverrides": overrides}
	}

	return values
}

// PrerequisiteChecks cover what the controller needs from the cluster rather
// than from the network. It makes no outbound connection, so unlike the other
// installers there is nothing to dial and no check pod to start.
func (opts *InstallOptions) PrerequisiteChecks() []octoK8s.Check {
	certManager := octoK8s.Check{Name: "cert-manager", Result: octoK8s.CheckPassed, Detail: "installed"}
	switch {
	case opts.CertManagerPresent:
	case !opts.CertManager.Value:
		certManager.Result = octoK8s.CheckSkipped
		certManager.Detail = "not installed, and the chart has been told not to use it"
	default:
		certManager.Result = octoK8s.CheckFailed
		certManager.Detail = "not installed"
		certManager.Remediation = fmt.Sprintf(
			"The controller's mutating admission webhook needs a certificate. Install cert-manager, or pass --%s=false "+
				"if you are supplying the certificate another way.", FlagCertManager)
	}

	agentCheck := octoK8s.Check{Name: "Kubernetes agents", Result: octoK8s.CheckPassed}
	if len(opts.Agents) == 0 {
		// Not a failure: the docs sanction installing the controller first, and
		// it simply does nothing until an agent turns up.
		agentCheck.Result = octoK8s.CheckSkipped
		agentCheck.Detail = fmt.Sprintf("none installed; the controller does nothing until an agent %s or newer arrives", MinimumAgentVersion)
	} else {
		agentCheck.Detail = fmt.Sprintf("%d found", len(opts.Agents))
	}

	return []octoK8s.Check{certManager, agentCheck}
}

func (opts *InstallOptions) confirmPrerequisites() error {
	if opts.SkipPreflight.Value {
		return nil
	}

	failed := shared.PrintChecks(opts.Out, "Prerequisites", opts.PrerequisiteChecks())
	if failed == 0 {
		return nil
	}

	// A dry run creates nothing, so an unmet prerequisite is worth reporting but
	// not worth withholding the preview for.
	if opts.DryRun.Value {
		return nil
	}

	if opts.NoPrompt {
		return fmt.Errorf("%d %s not met; fix the problems above or pass --%s",
			failed, octoK8s.Pluralise("prerequisite was", "prerequisites were", failed), octoK8s.FlagSkipPreflight)
	}

	proceed := false
	if err := opts.Ask(&survey.Confirm{
		Message: "Continue with the install anyway?",
		Default: false,
		Help:    "The controller is likely to install and then be unable to do its job.",
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("install cancelled")
	}
	return nil
}

func (opts *InstallOptions) renderOnly(ctx context.Context, values map[string]any, timeout time.Duration) error {
	fmt.Fprintf(opts.Out, "\n%s Rendering only. Nothing will be installed.\n", output.Dim("--"+octoK8s.FlagDryRun))

	manifest, err := opts.Runner.Render(ctx, helm.InstallSpec{
		Chart:       opts.chartRef(),
		ReleaseName: opts.TargetRelease,
		Namespace:   opts.TargetNamespace,
		Values:      values,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(opts.Out, manifest)
	return nil
}

func (opts *InstallOptions) reportSuccess(release helm.Release) {
	fmt.Fprintf(opts.Out, "\n%s Installed %s %s as release %s in namespace %s.\n",
		output.Green("✔"), release.Chart, release.Version,
		output.Cyan(release.Name), output.Cyan(release.Namespace))

	opts.PrintNextSteps()

	if opts.NoPrompt {
		return
	}

	generatable := []flag.Generatable{
		opts.TargetNamespaces, opts.TargetNamespaceRegex, opts.NamespacedRBAC,
		negated{name: FlagCertManager, off: !opts.CertManager.Value},
	}
	generatable = append(generatable, opts.CommonFlags.Generatable()...)

	autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), generatable...)
	fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
}

// PrintNextSteps exists because the controller changes nothing on its own:
// deployments carry on using the agent's default script pod permissions until a
// WorkloadServiceAccount matches them, and until each agent stops granting
// those defaults cluster-wide.
func (opts *InstallOptions) PrintNextSteps() {
	fmt.Fprintf(opts.Out, "\n%s\n", output.Bold("Next steps"))

	fmt.Fprintf(opts.Out, "\n  Create a %s in each namespace you deploy to, scoped to the deployments it applies\n"+
		"  to and naming the permissions they get:\n\n", output.Cyan("WorkloadServiceAccount"))
	fmt.Fprint(opts.Out, output.Dim(heredoc.Doc(`
	    apiVersion: agent.octopus.com/v1beta1
	    kind: WorkloadServiceAccount
	    metadata:
	      name: sample-wsa
	      namespace: your-application-namespace
	    spec:
	      scope:
	        spaces: [default]
	        projects: [guestbook]
	        environments: [dev-a, dev-b]
	      permissions:
	        permissions:
	          - verbs: ["*"]
	            apiGroups: ["*"]
	            resources: ["*"]
	`)))

	opts.printAgentRestrictions()
}

func (opts *InstallOptions) printAgentRestrictions() {
	unrestricted := make([]agent.Installation, 0, len(opts.Agents))
	for _, installation := range opts.Agents {
		if installation.ScriptPodClusterRole {
			unrestricted = append(unrestricted, installation)
		}
	}
	if len(unrestricted) == 0 {
		return
	}

	fmt.Fprintf(opts.Out, "\n  A deployment with no matching WorkloadServiceAccount falls back to the agent's default\n"+
		"  script pod permissions, which %s still grant across the cluster. Run this to take those\n"+
		"  defaults away, so an unmatched deployment fails instead:\n\n",
		octoK8s.Pluralise("this agent", "these agents", len(unrestricted)))

	for _, installation := range unrestricted {
		fmt.Fprintf(opts.Out, "  %s\n", output.Dimf("# %s", installation.Name))
		fmt.Fprintf(opts.Out, "  %s\n\n", output.Cyan(agentRestrictCommand(installation)))
	}
}

func agentRestrictCommand(installation agent.Installation) string {
	return fmt.Sprintf("helm upgrade --install --atomic --create-namespace --namespace %s --reset-then-reuse-values "+
		"--set scriptPods.serviceAccount.clusterRole.enabled=\"false\" %s %s",
		installation.Release.Namespace, installation.Release.Name, AgentChartRef)
}

// negated renders a flag that defaults to on. GenerateAutomationCmd emits a bool
// flag only when it is true, so turning one off has to be spelled out.
type negated struct {
	name string
	off  bool
}

func (n negated) GetName() string { return n.name + "=false" }
func (n negated) GetValue() any   { return n.off }
func (n negated) IsSecure() bool  { return false }

func (opts *InstallOptions) ReportSuccessForTest(release helm.Release) {
	opts.reportSuccess(release)
}

func (opts *InstallOptions) ConfirmPrerequisitesForTest() error {
	return opts.confirmPrerequisites()
}
