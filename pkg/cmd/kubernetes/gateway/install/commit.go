package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/argocdgateways"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"sigs.k8s.io/yaml"
)

func (opts *InstallOptions) Commit(ctx context.Context) error {
	timeout, err := opts.ResolveTimeout()
	if err != nil {
		return err
	}

	values, err := opts.BuildValues()
	if err != nil {
		return err
	}

	if err := opts.writeValuesFile(values); err != nil {
		return err
	}

	if err := opts.checkPermissions(ctx); err != nil {
		return err
	}

	if opts.DryRun.Value {
		return opts.renderOnly(ctx, values, timeout)
	}

	if err := opts.ensureNamespace(ctx); err != nil {
		return err
	}

	if err := opts.runPreflight(ctx); err != nil {
		return err
	}

	if err := opts.register(); err != nil {
		return err
	}

	if err := opts.storeCredentials(ctx); err != nil {
		return opts.deregister(err)
	}

	fmt.Fprintf(opts.Out, "\nInstalling the Argo CD gateway into %s...\n", output.Cyan(opts.TargetNamespace))
	if opts.Wait.Value || opts.Atomic.Value {
		// The gateway registers with Octopus from a job before it starts, so
		// there is a long quiet stretch here. Say so rather than look stalled.
		fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
			"Waiting for it to register with Octopus and become ready. This can take a few minutes, and gives up after %s.", timeout))
	}

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
		return opts.deregister(err)
	}

	opts.reportSuccess(release)
	return nil
}

// register obtains the gateway's own credential from Octopus, so the chart does
// not have to be given one of the user's to register itself with.
func (opts *InstallOptions) register() error {
	if opts.RegisterCallback == nil {
		return errors.New("no way to register the gateway with Octopus was configured")
	}

	environments := opts.Environments.Value
	if environments == nil {
		environments = []string{}
	}

	registration, err := opts.RegisterCallback(argocdgateways.RegisterCommand{
		SpaceID:      opts.Space.ID,
		Name:         opts.Name.Value,
		Environments: environments,
	})
	if err != nil {
		return err
	}
	if registration.AuthenticationToken == "" {
		return errors.New("Octopus registered the gateway but returned no credential for it")
	}

	opts.Registration = registration
	fmt.Fprintf(opts.Out, "%s Registered %s with Octopus\n", output.Green("✔"), output.Cyan(opts.Name.Value))
	return nil
}

// deregister removes the registration when the install it was for did not
// happen, so a failed attempt does not leave a gateway in Octopus that will
// never connect.
func (opts *InstallOptions) deregister(cause error) error {
	if opts.Registration == nil || opts.DeregisterCallback == nil {
		return cause
	}

	if err := opts.DeregisterCallback(opts.Registration.ID); err != nil {
		fmt.Fprintf(opts.Out, "%s The install failed, and the gateway registration in Octopus could not be removed: %v\n"+
			"  Delete %s under Infrastructure > Argo CD Instances before trying again.\n",
			output.Yellow("!"), err, output.Cyan(opts.Name.Value))
		return cause
	}

	fmt.Fprintf(opts.Out, "  %s\n", output.Dim("Removed the gateway registration from Octopus."))
	return cause
}

func (opts *InstallOptions) chartRef() helm.ChartRef {
	ref := ChartRef
	ref.Version = opts.ChartVersion.Value
	return ref
}

// BuildValues passes credentials by Secret reference unless --inline-secrets
// was given, so they stay out of the release values and out of any file written
// by --output-values.
func (opts *InstallOptions) BuildValues() (map[string]any, error) {
	if opts.ArgoCDServerGRPCURL.Value == "" {
		return nil, errors.New("the Argo CD in-cluster address could not be determined; specify --" + FlagArgoCDServerGRPCURL)
	}
	if opts.OctopusGRPCURL.Value == "" {
		return nil, errors.New("the Octopus gRPC address could not be determined; specify --" + FlagOctopusGRPCURL)
	}

	// register is off: Octopus registers the gateway before the chart is
	// installed, so no Octopus credential of the user's ever reaches the
	// cluster. The chart still wants these for its own configuration.
	registrationOctopus := map[string]any{
		"name":         opts.Name.Value,
		"serverApiUrl": opts.Host,
		"spaceId":      opts.Space.ID,
		"environments": opts.Environments.Value,
	}

	gatewayArgoCD := map[string]any{
		"serverGrpcUrl": opts.ArgoCDServerGRPCURL.Value,
		"plaintext":     opts.Instance.Plaintext,
		"insecure":      opts.Instance.SelfSignedTLS,
	}
	if opts.useGRPCWeb() {
		gatewayArgoCD["grpcWeb"] = true
	}
	if opts.ArgoCDGRPCWebRootPath.Value != "" {
		gatewayArgoCD["grpcWebRootPath"] = opts.ArgoCDGRPCWebRootPath.Value
	}

	projectTokens, err := opts.ProjectTokens()
	if err != nil {
		return nil, err
	}

	switch {
	case len(projectTokens) > 0:
		// AWS caps account tokens at 12 hours, so managed Argo CD authenticates
		// per project instead.
		if opts.InlineSecrets.Value {
			gatewayArgoCD["projectAuthentication"] = projectTokens
		} else {
			gatewayArgoCD["projectAuthenticationSecretName"] = projectTokenSecretName
		}
	case opts.InlineSecrets.Value:
		gatewayArgoCD["authenticationToken"] = opts.ArgoCDToken.Value
	default:
		gatewayArgoCD["authenticationTokenSecretName"] = argoTokenSecretName
		gatewayArgoCD["authenticationTokenSecretKey"] = argoTokenSecretKey
	}

	registration := map[string]any{"register": false, "octopus": registrationOctopus}
	if opts.ArgoCDWebUIURL.Value != "" {
		registration["argocd"] = map[string]any{"webUiUrl": opts.ArgoCDWebUIURL.Value}
	}

	return map[string]any{
		"registration": registration,
		"gateway": map[string]any{
			"argocd":  gatewayArgoCD,
			"octopus": map[string]any{"serverGrpcUrl": opts.OctopusGRPCURL.Value},
		},
	}, nil
}

func (opts *InstallOptions) writeValuesFile(values map[string]any) error {
	if opts.OutputValues.Value == "" {
		return nil
	}

	encoded, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("could not encode the Helm values: %w", err)
	}
	if err := os.WriteFile(opts.OutputValues.Value, encoded, 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", opts.OutputValues.Value, err)
	}

	fmt.Fprintf(opts.Out, "Wrote Helm values to %s\n", output.Cyan(opts.OutputValues.Value))
	if opts.InlineSecrets.Value {
		fmt.Fprintf(opts.Out, "%s This file contains credentials in plain text.\n", output.Yellow("!"))
	}
	return nil
}

// checkPermissions runs before anything is created, so a missing permission
// surfaces here rather than halfway through.
func (opts *InstallOptions) checkPermissions(ctx context.Context) error {
	denied, err := opts.Cluster.CheckPermissions(ctx, octoK8s.InstallPermissions(opts.TargetNamespace))
	if err != nil {
		return err
	}
	if len(denied) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "your Kubernetes credentials cannot perform this install in context %q:", opts.Cluster.ContextName)
	for _, d := range denied {
		fmt.Fprintf(&b, "\n  cannot %s - needed to %s", d, d.Description)
	}

	// A dry run creates nothing, so this is worth knowing but not worth
	// withholding the preview for.
	if opts.DryRun.Value {
		fmt.Fprintf(opts.Out, "%s %s\n", output.Yellow("!"), b.String())
		return nil
	}
	return errors.New(b.String())
}

func (opts *InstallOptions) ensureNamespace(ctx context.Context) error {
	exists, err := opts.Cluster.NamespaceExists(ctx, opts.TargetNamespace)
	if err != nil {
		return err
	}
	if exists {
		fmt.Fprintf(opts.Out, "Using existing namespace %s\n", output.Cyan(opts.TargetNamespace))
		return nil
	}
	return opts.Cluster.CreateNamespace(ctx, opts.TargetNamespace)
}

// runPreflight catches the case where an install succeeds and then never
// connects: the gateway registers over the REST API but runs over gRPC on a
// different port.
func (opts *InstallOptions) runPreflight(ctx context.Context) error {
	if opts.SkipPreflight.Value {
		return nil
	}

	targets := []octoK8s.Target{
		{
			Name:    "Octopus REST API",
			Address: opts.Host,
			Remediation: "The gateway registers itself with Octopus over the REST API. " +
				"Confirm this address is reachable from inside the cluster.",
		},
		{
			Name:    "Octopus gRPC endpoint",
			Address: opts.OctopusGRPCURL.Value,
			Remediation: "The running gateway connects to Octopus over gRPC on a different port to the REST API. " +
				"A load balancer, proxy, or firewall that forwards only HTTPS is the usual cause; make sure the gRPC port is forwarded too.",
		},
	}

	checks := octoK8s.StaticChecks(targets)
	podChecks, err := opts.Cluster.RunPreflight(ctx, octoK8s.PreflightRequest{
		Namespace: opts.TargetNamespace,
		Image:     opts.PreflightImage.Value,
		Targets:   targets,
	})
	if err != nil {
		return err
	}
	checks = append(checks, podChecks...)

	return opts.confirmPreflight(checks)
}

func (opts *InstallOptions) printPreflight(checks []octoK8s.Check) int {
	if len(checks) == 0 {
		return 0
	}

	fmt.Fprintln(opts.Out, "\nConnectivity checks:")
	failed := 0
	for _, c := range checks {
		switch c.Result {
		case octoK8s.CheckPassed:
			fmt.Fprintf(opts.Out, "  %s %s %s\n", output.Green("✔"), c.Name, output.Dim(c.Detail))
		case octoK8s.CheckSkipped:
			fmt.Fprintf(opts.Out, "  %s %s %s\n", output.Dim("-"), c.Name, output.Dim(c.Detail))
		default:
			failed++
			fmt.Fprintf(opts.Out, "  %s %s %s\n", output.Red("✘"), c.Name, c.Detail)
			if c.Remediation != "" {
				fmt.Fprintf(opts.Out, "      %s\n", output.Dim(c.Remediation))
			}
		}
	}

	return failed
}

func (opts *InstallOptions) confirmPreflight(checks []octoK8s.Check) error {
	failed := opts.printPreflight(checks)
	if failed == 0 {
		return nil
	}

	if opts.NoPrompt {
		return fmt.Errorf("%d connectivity %s failed; fix the problems above or pass --%s",
			failed, octoK8s.Pluralise("check", "checks", failed), octoK8s.FlagSkipPreflight)
	}

	// A check can be wrong: egress policy may allow the real workload's service
	// account but not a bare pod.
	proceed := false
	if err := opts.Ask(&survey.Confirm{
		Message: "Continue with the install anyway?",
		Default: false,
		Help:    "The gateway is likely to install and then fail to connect.",
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("install cancelled")
	}
	return nil
}

// storeCredentials keeps credentials out of the Helm release values.
func (opts *InstallOptions) storeCredentials(ctx context.Context) error {
	if opts.InlineSecrets.Value {
		return nil
	}

	projectTokens, err := opts.ProjectTokens()
	if err != nil {
		return err
	}

	if len(projectTokens) > 0 {
		// The chart reads these as environment variables prefixed with
		// OCTOPUS_ARGOCD_, so the key names are part of its contract.
		data := make(map[string]string, len(projectTokens))
		for _, t := range projectTokens {
			data["PROJECT_AUTH_TOKEN_"+t.Project] = t.Token
		}
		if err := opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, projectTokenSecretName, data); err != nil {
			return err
		}
	} else if err := opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, argoTokenSecretName, map[string]string{
		argoTokenSecretKey: opts.ArgoCDToken.Value,
	}); err != nil {
		return err
	}

	return opts.storeRegistration(ctx)
}

// storeRegistration writes the gateway's own credential in the shape the chart
// expects, taking the place of the registration job it would otherwise run.
func (opts *InstallOptions) storeRegistration(ctx context.Context) error {
	contents, err := opts.registrationSecretContents()
	if err != nil {
		return err
	}

	return opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, registrationSecretName,
		map[string]string{registrationSecretKey: contents})
}

func (opts *InstallOptions) registrationSecretContents() (string, error) {
	if opts.Registration == nil {
		return "", errors.New("the gateway was not registered with Octopus")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "octopus-grpc-authentication-token: %q\n", opts.Registration.AuthenticationToken)
	fmt.Fprintf(&b, "octopus-grpc-client-id: %q\n", opts.Registration.ClientID)
	if thumbprint := opts.Registration.Thumb(); thumbprint != "" {
		fmt.Fprintf(&b, "octopus-grpc-thumbprint: %q\n", thumbprint)
	}
	return b.String(), nil
}

func (opts *InstallOptions) renderOnly(ctx context.Context, values map[string]any, timeout time.Duration) error {
	fmt.Fprintf(opts.Out, "\n%s Rendering only. Nothing will be installed, and the connectivity checks that need a pod in the cluster are skipped.\n",
		output.Dim("--"+octoK8s.FlagDryRun))

	if !opts.SkipPreflight.Value {
		// Report only: there is no install to abandon.
		opts.printPreflight(octoK8s.StaticChecks([]octoK8s.Target{
			{Name: "Octopus REST API", Address: opts.Host},
			{Name: "Octopus gRPC endpoint", Address: opts.OctopusGRPCURL.Value},
		}))
	}

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
	fmt.Fprintf(opts.Out, "  The gateway registers itself with Octopus, then connects. "+
		"It appears under Infrastructure > Argo CD Instances once it is healthy.\n")

	if opts.NoPrompt {
		return
	}

	generatable := []flag.Generatable{
		opts.Name, opts.Environments, opts.ArgoCDNamespace, opts.ArgoCDServerGRPCURL,
		opts.ArgoCDToken, opts.ArgoCDProjectTokens, opts.ArgoCDWebUIURL, opts.OctopusGRPCURL,
		opts.ArgoCDGRPCWeb, opts.ArgoCDGRPCWebRootPath,
		opts.ArgoCDAccountName, opts.AllowSync, opts.InlineSecrets,
	}
	generatable = append(generatable, opts.CommonFlags.Generatable()...)

	autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), generatable...)
	fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
}

// ErrForTest is a sentinel used by tests to check error propagation.
var ErrForTest = errors.New("install failed")

// RegistrationSecretForTest renders the credential file written for the chart.
func (opts *InstallOptions) RegistrationSecretForTest() (string, error) {
	return opts.registrationSecretContents()
}

// RegisterForTest exposes the registration step.
func (opts *InstallOptions) RegisterForTest() error { return opts.register() }

// DeregisterForTest exposes the rollback step.
func (opts *InstallOptions) DeregisterForTest(cause error) error { return opts.deregister(cause) }
