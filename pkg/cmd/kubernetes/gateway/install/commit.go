package install

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OctopusDeploy/cli/pkg/argocdgateways"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	gatewayK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/gateway"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
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

	if err := shared.CheckPermissions(ctx, opts.Dependencies, opts.Cluster, octoK8s.InstallPermissions(opts.TargetNamespace), opts.DryRun.Value); err != nil {
		return err
	}

	if opts.DryRun.Value {
		return opts.renderOnly(ctx, values, timeout)
	}

	if err := shared.EnsureNamespace(ctx, opts.Dependencies, opts.Cluster, opts.TargetNamespace); err != nil {
		return err
	}

	if err := opts.preflight().Run(ctx); err != nil {
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

	release, err := opts.Runner.Install(ctx, opts.installSpec(values, timeout))
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
	return gatewayK8s.ChartRef.WithVersion(opts.ChartVersion.Value)
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
			gatewayArgoCD["projectAuthenticationSecretName"] = gatewayK8s.ProjectTokenSecretName
		}
	case opts.InlineSecrets.Value:
		gatewayArgoCD["authenticationToken"] = opts.ArgoCDToken.Value
	default:
		gatewayArgoCD["authenticationTokenSecretName"] = gatewayK8s.ArgoTokenSecretName
		gatewayArgoCD["authenticationTokenSecretKey"] = gatewayK8s.ArgoTokenSecretKey
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
	warning := ""
	if opts.InlineSecrets.Value {
		warning = "This file contains credentials in plain text."
	}
	return shared.WriteValuesFile(opts.Out, opts.OutputValues.Value, values, warning)
}

func (opts *InstallOptions) preflight() *shared.Preflight {
	return &shared.Preflight{
		Dependencies: opts.Dependencies,
		CommonFlags:  opts.CommonFlags,
		Cluster:      opts.Cluster,
		Namespace:    opts.TargetNamespace,
		Targets: []octoK8s.Target{
			octoK8s.RESTAPITarget(opts.Host, "The gateway registers itself with Octopus over the REST API."),
			octoK8s.GRPCTarget(opts.OctopusGRPCURL.Value,
				"The running gateway connects to Octopus over gRPC on a different port to the REST API."),
		},
		ProceedHelp: "The gateway is likely to install and then fail to connect.",
	}
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
			data[gatewayK8s.ProjectTokenKeyPrefix+t.Project] = t.Token
		}
		if err := opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, gatewayK8s.ProjectTokenSecretName, data); err != nil {
			return err
		}
	} else if err := opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, gatewayK8s.ArgoTokenSecretName, map[string]string{
		gatewayK8s.ArgoTokenSecretKey: opts.ArgoCDToken.Value,
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

	return opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, gatewayK8s.RegistrationSecretName,
		map[string]string{gatewayK8s.RegistrationSecretKey: contents})
}

func (opts *InstallOptions) registrationSecretContents() (string, error) {
	if opts.Registration == nil {
		return "", errors.New("the gateway was not registered with Octopus")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "octopus-grpc-authentication-token: %q\n", opts.Registration.AuthenticationToken)
	fmt.Fprintf(&b, "octopus-grpc-client-id: %q\n", opts.Registration.ClientID)
	if thumbprint := opts.Registration.CertificateThumbprint; thumbprint != "" {
		fmt.Fprintf(&b, "octopus-grpc-thumbprint: %q\n", thumbprint)
	}
	return b.String(), nil
}

func (opts *InstallOptions) renderOnly(ctx context.Context, values map[string]any, timeout time.Duration) error {
	return shared.RenderOnly(ctx, opts.Dependencies, opts.Runner, opts.installSpec(values, timeout),
		"Nothing will be installed, and the connectivity checks that need a pod in the cluster are skipped.",
		opts.preflight())
}

func (opts *InstallOptions) installSpec(values map[string]any, timeout time.Duration) helm.InstallSpec {
	return helm.InstallSpec{
		Chart:       opts.chartRef(),
		ReleaseName: opts.TargetRelease,
		Namespace:   opts.TargetNamespace,
		Values:      values,
		Atomic:      opts.Atomic.Value,
		Wait:        opts.Wait.Value,
		Timeout:     timeout,
	}
}

func (opts *InstallOptions) reportSuccess(release helm.Release) {
	shared.ReportInstalled(opts.Out, release)
	fmt.Fprintf(opts.Out, "  The gateway registers itself with Octopus, then connects. "+
		"It appears under Infrastructure > Argo CD Instances once it is healthy.\n")

	generatable := []flag.Generatable{
		opts.Name, opts.Environments, opts.ArgoCDNamespace, opts.ArgoCDServerGRPCURL,
		opts.ArgoCDToken, opts.ArgoCDProjectTokens, opts.ArgoCDWebUIURL, opts.OctopusGRPCURL,
		opts.ArgoCDGRPCWeb, opts.ArgoCDGRPCWebRootPath,
		opts.ArgoCDAccountName, opts.AllowSync, opts.InlineSecrets,
	}
	generatable = append(generatable, opts.CommonFlags.Generatable()...)
	shared.PrintAutomationCommand(opts.Dependencies, generatable)
}
