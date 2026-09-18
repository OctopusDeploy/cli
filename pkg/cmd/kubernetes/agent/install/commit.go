package install

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

func (opts *InstallOptions) Commit(ctx context.Context) error {
	timeout, err := opts.ResolveTimeout()
	if err != nil {
		return err
	}

	if err := shared.CheckPermissions(ctx, opts.Dependencies, opts.Cluster, octoK8s.InstallPermissions(opts.TargetNamespace), opts.DryRun.Value); err != nil {
		return err
	}

	if opts.DryRun.Value {
		return opts.renderOnly(ctx, timeout)
	}

	if err := shared.EnsureNamespace(ctx, opts.Dependencies, opts.Cluster, opts.TargetNamespace); err != nil {
		return err
	}

	if err := opts.preflight().Run(ctx); err != nil {
		return err
	}

	// Asked before the chart runs, so a registration that appears afterwards can
	// be told apart from one that was always there.
	_, _ = opts.alreadyRegistered()

	if err := opts.authenticate(ctx); err != nil {
		return err
	}

	values, err := opts.BuildValues()
	if err != nil {
		return err
	}
	if err := opts.writeValuesFile(values); err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "\nInstalling the Octopus %s into %s...\n", opts.installedThing(), output.Cyan(opts.TargetNamespace))
	if opts.Wait.Value || opts.Atomic.Value {
		// The chart registers the agent from a pod before the agent itself
		// starts, so there is a long quiet stretch here. Say so rather than
		// look stalled.
		fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
			"Waiting for it to register with Octopus and become ready. This can take a few minutes, and gives up after %s.", timeout))
	}

	release, err := opts.Runner.Install(ctx, opts.installSpec(values, timeout))
	if err != nil {
		return opts.reportFailure(err)
	}

	opts.reportSuccess(release)
	return nil
}

// authenticate gets the credential the chart registers the agent with. An
// access token is used rather than an API key because it expires within the
// hour, so nothing worth stealing is left in the cluster afterwards.
func (opts *InstallOptions) authenticate(ctx context.Context) error {
	if opts.AccessTokenCallback == nil {
		return errors.New("no way to get an Octopus access token was configured")
	}

	token, err := opts.AccessTokenCallback()
	if err != nil {
		return err
	}
	opts.Token = token

	if opts.InlineSecrets.Value {
		return nil
	}
	return opts.Cluster.UpsertSecret(ctx, opts.TargetNamespace, tokenSecretName,
		map[string]string{tokenSecretKey: token.Value})
}

// BuildValues passes the credential by Secret reference unless
// --inline-secrets was given, so it stays out of the release values and out of
// any file written by --output-values.
func (opts *InstallOptions) BuildValues() (map[string]any, error) {
	if len(opts.ServerCommsAddresses.Value) == 0 {
		return nil, errors.New("the Octopus polling address could not be determined; specify --" + FlagServerCommsAddress)
	}

	agentValues := map[string]any{
		"name":       opts.Name.Value,
		"acceptEula": eulaValue(opts.AcceptEula.Value),
		"serverUrl":  opts.Host,
		// The list form of the polling address, as the portal's generated
		// command uses; a High Availability cluster has one entry per node.
		"serverCommsAddresses": opts.ServerCommsAddresses.Value,
		"space":                opts.spaceName(),
	}

	if opts.MachinePolicy.Value != "" {
		agentValues["machinePolicyName"] = opts.MachinePolicy.Value
	}
	if opts.ServerCertificate.Value != "" {
		agentValues["serverCertificate"] = opts.ServerCertificate.Value
	}

	// A dry run never asks Octopus for a token, so there is nothing to inline
	// and the Secret reference is what the real install would use anyway.
	if opts.InlineSecrets.Value && opts.Token.Value != "" {
		agentValues["bearerToken"] = opts.Token.Value
	} else {
		agentValues["bearerTokenSecretName"] = tokenSecretName
	}

	registration, err := opts.registrationValues()
	if err != nil {
		return nil, err
	}
	maps.Copy(agentValues, registration)

	values := map[string]any{"agent": agentValues}

	if persistence := opts.persistenceValues(); len(persistence) > 0 {
		values["persistence"] = persistence
	}
	if scriptPods := opts.scriptPodValues(); len(scriptPods) > 0 {
		values["scriptPods"] = scriptPods
	}
	if opts.monitorEnabled() {
		values["kubernetesMonitor"] = opts.monitorValues()
	}

	return values, nil
}

// monitorEnabled guards on the mode as well as the flag: the monitor watches
// the objects deployments create, which only a deployment target has.
func (opts *InstallOptions) monitorEnabled() bool {
	return opts.KubernetesMonitor.Value && !opts.isWorker()
}

// monitorValues fills in the monitor subchart the same way the Octopus portal
// does. It registers with the same short-lived token as the agent, so the two
// share one Secret.
func (opts *InstallOptions) monitorValues() map[string]any {
	registration := map[string]any{
		"serverApiUrl": opts.Host,
		"spaceId":      opts.spaceID(),
		"machineName":  opts.Name.Value,
	}
	if opts.InlineSecrets.Value && opts.Token.Value != "" {
		registration["serverAccessToken"] = opts.Token.Value
	} else {
		registration["serverAccessTokenSecretName"] = tokenSecretName
		registration["serverAccessTokenSecretKey"] = tokenSecretKey
	}
	if opts.ServerCertificate.Value != "" {
		registration["serverCertificate"] = opts.ServerCertificate.Value
	}

	return map[string]any{
		"enabled":      true,
		"monitor":      map[string]any{"serverGrpcUrl": octoK8s.DeriveGRPCURL(opts.Host)},
		"registration": registration,
	}
}

// TargetTags is the one list of tags the agent registers with. Octopus builds it
// from plain role names and from tag set entries, which are given in canonical
// TagSetName/TagName form and sent as the tag name alone.
func (opts *InstallOptions) TargetTags() ([]string, error) {
	return sharedTarget.CombineRolesAndTags(opts.Client, opts.Roles.Value, opts.Tags.Value)
}

// scriptPodValues decides what a deployment can do when no
// WorkloadServiceAccount matches it. Left out entirely, the chart grants script
// pods the whole cluster.
func (opts *InstallOptions) scriptPodValues() map[string]any {
	clusterRole := map[string]any{}
	switch {
	case opts.RestrictScriptPods.Value:
		// No ClusterRole at all is what makes an unmatched deployment fail
		// rather than run with more access than it should have.
		clusterRole["enabled"] = false
	case len(opts.ScriptPodRules) > 0:
		clusterRole["rules"] = opts.ScriptPodRules
	default:
		return nil
	}

	return map[string]any{"serviceAccount": map[string]any{"clusterRole": clusterRole}}
}

// registrationValues fills in one half of the chart: an agent registers as a
// deployment target or as a worker, never both.
func (opts *InstallOptions) registrationValues() (map[string]any, error) {
	if opts.isWorker() {
		return map[string]any{
			"worker": map[string]any{
				"enabled": true,
				"initial": map[string]any{"workerPools": opts.WorkerPools.Value},
			},
		}, nil
	}

	tags, err := opts.TargetTags()
	if err != nil {
		return nil, err
	}

	initial := map[string]any{
		"environments":                    opts.Environments.Value,
		"tags":                            tags,
		"tenantedDeploymentParticipation": opts.tenantedParticipation(),
	}
	if opts.DefaultNamespace.Value != "" {
		initial["defaultNamespace"] = opts.DefaultNamespace.Value
	}
	if len(opts.Tenants.Value) > 0 {
		initial["tenants"] = opts.Tenants.Value
	}
	if len(opts.TenantTags.Value) > 0 {
		initial["tenantTags"] = opts.TenantTags.Value
	}

	return map[string]any{
		"deploymentTarget": map[string]any{"enabled": true, "initial": initial},
	}, nil
}

func (opts *InstallOptions) tenantedParticipation() string {
	if opts.TenantedDeploymentMode.Value == "" {
		return sharedTarget.Untenanted
	}
	return opts.TenantedDeploymentMode.Value
}

// persistenceValues is left out entirely unless something was chosen, so the
// chart's own defaults apply: the cluster's default storage class, mounted
// ReadWriteOnce.
func (opts *InstallOptions) persistenceValues() map[string]any {
	persistence := map[string]any{}
	if opts.StorageClass.Value != "" {
		persistence["storageClassName"] = opts.StorageClass.Value
	}
	if opts.ReadWriteMany.Value {
		persistence["accessModes"] = []string{"ReadWriteMany"}
	}
	return persistence
}

func eulaValue(accepted bool) string {
	if accepted {
		return "Y"
	}
	return "N"
}

func (opts *InstallOptions) preflight() *shared.Preflight {
	targets := []octoK8s.Target{
		octoK8s.RESTAPITarget(opts.Host, "The chart registers the agent with Octopus over the REST API, from a pod in the cluster."),
	}

	for _, address := range opts.ServerCommsAddresses.Value {
		targets = append(targets, octoK8s.Target{
			Name:    "Octopus polling endpoint",
			Address: address,
			Remediation: fmt.Sprintf("The running agent polls Octopus over TCP on this address, on port %d by default and separately from the REST API. "+
				"A firewall or proxy that only allows HTTPS is the usual cause. The connection also has to reach Octopus intact, so SSL offloading will not work.",
				octoK8s.DefaultPollingPort),
		})
	}

	if opts.monitorEnabled() {
		targets = append(targets, octoK8s.GRPCTarget(octoK8s.DeriveGRPCURL(opts.Host),
			"The Kubernetes monitor streams live object status to Octopus over gRPC on a different port to the REST API."))
	}

	return &shared.Preflight{
		Dependencies: opts.Dependencies,
		CommonFlags:  opts.CommonFlags,
		Cluster:      opts.Cluster,
		Namespace:    opts.TargetNamespace,
		Targets:      targets,
		ProceedHelp:  "The agent is likely to install and then fail to register.",
	}
}

func (opts *InstallOptions) writeValuesFile(values map[string]any) error {
	warning := ""
	if opts.InlineSecrets.Value && opts.Token.Value != "" {
		warning = "This file contains an Octopus access token in plain text."
	}
	return shared.WriteValuesFile(opts.Out, opts.OutputValues.Value, values, warning)
}

func (opts *InstallOptions) renderOnly(ctx context.Context, timeout time.Duration) error {
	// The rendered manifests carry acceptEula, and an agent given "N" starts and
	// then refuses to run, so values taken from here would not work as they are.
	if !opts.AcceptEula.Value {
		fmt.Fprintf(opts.Out, "%s These values decline the Octopus Customer Agreement. Add --%s to render values that can be installed.\n",
			output.Yellow("!"), FlagAcceptEula)
	}

	values, err := opts.BuildValues()
	if err != nil {
		return err
	}
	if err := opts.writeValuesFile(values); err != nil {
		return err
	}

	return shared.RenderOnly(ctx, opts.Dependencies, opts.Runner, opts.installSpec(values, timeout),
		"Nothing will be installed, no Octopus access token is created, and the connectivity checks that need a pod in the cluster are skipped.",
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

// reportFailure covers the case Helm cannot undo. The chart registers the agent
// from a pre-install hook, so a rollback leaves the registration behind, and an
// agent in Octopus that will never connect is worse than none.
func (opts *InstallOptions) reportFailure(cause error) error {
	if opts.RegisteredCallback == nil || opts.registeredBefore {
		return cause
	}

	opts.registrationCheckedFor = ""
	taken, err := opts.alreadyRegistered()
	if err != nil || !taken {
		return cause
	}

	fmt.Fprintf(opts.Out, "\n%s The install failed after the agent had registered itself, so Octopus now has a %s named %s that will never connect.\n",
		output.Yellow("!"), opts.Mode, output.Cyan(opts.Name.Value))
	fmt.Fprintf(opts.Out, "  Remove it with %s before trying again.\n", output.Cyan(opts.removeCommand()))
	return cause
}

func (opts *InstallOptions) removeCommand() string {
	if opts.isWorker() {
		return fmt.Sprintf("%s worker delete %q", constants.ExecutableName, opts.Name.Value)
	}
	return fmt.Sprintf("%s deployment-target delete %q", constants.ExecutableName, opts.Name.Value)
}

func (opts *InstallOptions) reportSuccess(release helm.Release) {
	shared.ReportInstalled(opts.Out, release)
	fmt.Fprintf(opts.Out, "  The agent polls Octopus for work. It appears under %s once its first health check passes.\n",
		opts.portalLocation())

	shared.PrintAutomationCommand(opts.Dependencies, opts.generatable())
}

func (opts *InstallOptions) portalLocation() string {
	if opts.isWorker() {
		return "Infrastructure > Worker Pools"
	}
	return "Infrastructure > Deployment Targets"
}

func (opts *InstallOptions) generatable() []flag.Generatable {
	generatable := []flag.Generatable{opts.Name}

	if opts.isWorker() {
		generatable = append(generatable, opts.WorkerPools)
	} else {
		generatable = append(generatable, opts.Environments, opts.Roles, opts.Tags,
			opts.TenantedDeploymentMode, opts.Tenants, opts.TenantTags, opts.DefaultNamespace,
			opts.KubernetesMonitor)
	}

	generatable = append(generatable, opts.MachinePolicy, opts.ServerCommsAddresses, opts.ServerCertificate,
		opts.StorageClass, opts.ReadWriteMany, opts.AcceptEula, opts.InlineSecrets,
		opts.RestrictScriptPods, opts.ScriptPodRoles)
	return append(generatable, opts.CommonFlags.Generatable()...)
}
