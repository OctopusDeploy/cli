package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
)

// Confirm shows every setting, detected or chosen. Most are worked out rather
// than asked for, which is the point of the wizard, but it also means nobody
// sees them unless they are shown.
func Confirm(ctx context.Context, opts *InstallOptions) error {
	review := &shared.Review{
		Dependencies: opts.Dependencies,
		Groups:       func() []shared.Group { return reviewGroups(opts) },
		Refresh: func() error {
			// The namespace and release name follow the agent's name, and the
			// access mode follows the storage class, so an edit to either has to
			// carry through.
			opts.deriveAccessMode()
			return opts.resolveNames()
		},
	}
	return review.Confirm(ctx)
}

func reviewGroups(opts *InstallOptions) []shared.Group {
	groups := []shared.Group{
		{Title: "Cluster", Items: clusterItems(opts)},
		{Title: "Octopus", Items: octopusItems(opts)},
	}

	if opts.isWorker() {
		groups = append(groups, shared.Group{Title: "Worker", Items: workerItems(opts)})
	} else {
		groups = append(groups, shared.Group{Title: "Deployment target", Items: deploymentTargetItems(opts)})
	}

	return append(groups,
		shared.Group{Title: "Script pods", Items: scriptPodItems(opts)},
		shared.Group{Title: "Helm", Items: helmItems(opts)},
	)
}

func clusterItems(opts *InstallOptions) []shared.Item {
	source := "current context"
	if opts.KubeContext.Value != "" && !opts.KubeContextInfo.IsCurrent {
		source = "chosen"
	}

	return []shared.Item{
		{
			Label:  "Kubernetes context",
			Value:  opts.KubeContext.Value,
			Source: source,
			// Changing cluster invalidates everything discovered from it.
			Edit: nil,
		},
		{Label: "Cluster address", Value: opts.KubeContextInfo.Server, Source: "from the kubeconfig"},
		{Label: "Node architectures", Value: shared.OrNone(opts.NodeArchitectures), Source: "from the cluster"},
		{
			Label:  "Namespace",
			Value:  opts.TargetNamespace,
			Source: shared.DerivedOrSet(opts.Namespace.Value, "derived from the name"),
			Edit: shared.EditText(opts.Ask, &opts.Namespace.Value, "Namespace to install into",
				func() string { return opts.TargetNamespace }),
		},
		{
			Label:  "Helm release",
			Value:  opts.TargetRelease,
			Source: shared.DerivedOrSet(opts.ReleaseName.Value, "derived from the name"),
			Edit: shared.EditText(opts.Ask, &opts.ReleaseName.Value, "Helm release name",
				func() string { return opts.TargetRelease }),
		},
	}
}

func octopusItems(opts *InstallOptions) []shared.Item {
	return []shared.Item{
		{
			Label: "Name", Value: opts.Name.Value, Source: "chosen",
			Edit: func(context.Context) error {
				opts.Name.Value = ""
				return question.AskName(opts.Ask, "", string(opts.Mode), &opts.Name.Value)
			},
		},
		{Label: "Server", Value: opts.Host, Source: "from your login"},
		{Label: "Space", Value: opts.spaceName(), Source: "from your login"},
		{
			Label:  pollingLabel(opts),
			Value:  strings.Join(opts.ServerCommsAddresses.Value, ", "),
			Source: pollingSource(opts),
			Edit: func(context.Context) error {
				opts.ServerCommsAddresses.Value = nil
				return promptForPollingAddresses(opts)
			},
		},
		{Label: "Registration", Value: registrationSummary(opts), Source: opts.Token.Describe()},
		{
			Label: "Machine policy", Value: shared.OrDefault(opts.MachinePolicy.Value, "(default)"), Source: "",
			Edit: shared.EditText(opts.Ask, &opts.MachinePolicy.Value, "Machine policy name (blank for the default)",
				func() string { return opts.MachinePolicy.Value }),
		},
		{
			Label: "Server certificate", Value: certificateSummary(opts), Source: "",
			Edit: shared.EditText(opts.Ask, &opts.ServerCertificate.Value, "Base64-encoded PEM certificate to trust (blank for none)",
				func() string { return opts.ServerCertificate.Value }),
		},
	}
}

func deploymentTargetItems(opts *InstallOptions) []shared.Item {
	items := []shared.Item{
		{
			Label: "Environments", Value: shared.OrNone(opts.Environments.Value), Source: "chosen",
			Edit: func(context.Context) error {
				opts.Environments.Value = nil
				return sharedTarget.PromptForEnvironments(opts.CreateTargetEnvironmentOptions, opts.CreateTargetEnvironmentFlags)
			},
		},
		{
			Label:  "Target tags",
			Value:  shared.OrNone(append(append([]string{}, opts.Roles.Value...), opts.Tags.Value...)),
			Source: targetTagSource(opts),
			Edit: func(context.Context) error {
				opts.Roles.Value = nil
				opts.Tags.Value = nil
				return promptForTargetTags(opts)
			},
		},
		{
			Label: "Default namespace", Value: shared.OrNotSet(opts.DefaultNamespace.Value), Source: "",
			Edit: shared.EditText(opts.Ask, &opts.DefaultNamespace.Value, "Default namespace for deployments (optional)",
				func() string { return opts.DefaultNamespace.Value }),
		},
		{
			Label: "Tenanted deployments", Value: tenantedSummary(opts), Source: "",
			Edit: editTenantedParticipation(opts),
		},
	}

	if len(opts.Tenants.Value) > 0 || len(opts.TenantTags.Value) > 0 {
		items = append(items, shared.Item{
			Label:  "Tenants",
			Value:  shared.OrNone(append(append([]string{}, opts.Tenants.Value...), opts.TenantTags.Value...)),
			Source: "chosen",
		})
	}

	if opts.monitorEnabled() {
		items = append(items, shared.Item{
			Label:  "Kubernetes monitor",
			Value:  "installed alongside the agent",
			Source: "streams live status of deployed objects to Octopus over gRPC",
		})
	}
	return items
}

func workerItems(opts *InstallOptions) []shared.Item {
	return []shared.Item{
		{
			Label: "Worker pools", Value: shared.OrNone(opts.WorkerPools.Value), Source: "chosen",
			Edit: func(context.Context) error {
				opts.WorkerPools.Value = nil
				return sharedWorker.PromptForWorkerPools(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
			},
		},
	}
}

func scriptPodItems(opts *InstallOptions) []shared.Item {
	items := []shared.Item{
		{
			Label: "Storage class", Value: shared.OrDefault(opts.StorageClass.Value, "(cluster default)"),
			Source: storageSource(opts),
			Edit: func(context.Context) error {
				opts.StorageClass.Value = ""
				if len(opts.StorageClasses) == 0 {
					return nil
				}
				return promptForStorageClass(opts)
			},
		},
		{
			Label: "Access mode", Value: accessModeSummary(opts), Source: accessModeSource(opts),
			Edit: func(ctx context.Context) error {
				// Answering makes it a choice, so a later change of storage class
				// does not quietly take it back.
				opts.AccessModeChosen = true
				return shared.EditConfirm(opts.Ask, &opts.ReadWriteMany.Value,
					"Let script pods run on any node?",
					"This asks for a ReadWriteMany volume, which only works with a storage class that serves a shared filesystem.")(ctx)
			},
		},
	}

	if opts.PermissionsController || opts.RestrictScriptPods.Value || len(opts.ScriptPodRoles.Value) > 0 {
		items = append(items, shared.Item{
			Label: "Permissions", Value: permissionsSummary(opts), Source: permissionsSource(opts),
			Edit: func(context.Context) error {
				opts.RestrictScriptPods.Value = false
				opts.ScriptPodRoles.Value = nil
				opts.ScriptPodRules = nil
				return promptForScriptPodPermissions(opts)
			},
		})
	}
	return items
}

func helmItems(opts *InstallOptions) []shared.Item {
	return []shared.Item{
		{Label: "Chart", Value: ChartRef.Ref},
		{
			Label: "Chart version", Value: shared.OrDefault(opts.ChartVersion.Value, ChartRef.Version),
			Edit: shared.EditText(opts.Ask, &opts.ChartVersion.Value, fmt.Sprintf("Chart version (blank for %s)", ChartRef.Version),
				func() string { return opts.ChartVersion.Value }),
		},
		{
			Label: "Credentials", Value: credentialPlacement(opts),
			Edit: shared.EditConfirm(opts.Ask, &opts.InlineSecrets.Value,
				"Put the registration credential directly in the Helm values instead of a Kubernetes Secret?",
				"A Secret keeps it out of the Helm release and out of any file written with --output-values."),
		},
		{
			Label: "Timeout", Value: shared.OrDefault(opts.Timeout.Value, octoK8s.DefaultTimeout.String()),
			Edit: shared.EditText(opts.Ask, &opts.Timeout.Value, "How long to wait for the release to become ready",
				func() string { return opts.Timeout.Value }),
		},
	}
}

func registrationSummary(opts *InstallOptions) string {
	return fmt.Sprintf("the agent registers itself as a %s", opts.Mode)
}

func pollingLabel(opts *InstallOptions) string {
	if len(opts.ServerCommsAddresses.Value) > 1 {
		return "Polling addresses"
	}
	return "Polling address"
}

func pollingSource(opts *InstallOptions) string {
	if len(opts.ServerCommsAddresses.Value) > 1 {
		return "one per Octopus Server node"
	}
	return "derived from the server address"
}

func certificateSummary(opts *InstallOptions) string {
	if opts.ServerCertificate.Value == "" {
		return "(publicly trusted)"
	}
	return "supplied"
}

// targetTagSource calls out a tag Octopus has never seen, because choosing one
// creates it rather than matching anything that exists.
func targetTagSource(opts *InstallOptions) string {
	created := opts.newTargetTags()
	if len(created) == 0 {
		return "chosen"
	}
	return fmt.Sprintf("%s created when the agent registers", strings.Join(created, ", "))
}

// editTenantedParticipation asks only which kinds of deployment the target
// takes part in. Which tenants it serves is left to the target's settings in
// Octopus, which is where the agent's own documentation sends people, and to
// --tenant and --tenant-tag.
func editTenantedParticipation(opts *InstallOptions) func(context.Context) error {
	return func(context.Context) error {
		selected, err := selectors.SelectOptions(opts.Ask,
			"Choose the kind of deployments where this deployment target should be included",
			sharedTarget.TenantDeploymentOptions)
		if err != nil {
			return err
		}
		opts.TenantedDeploymentMode.Value = selected.Value
		return nil
	}
}

func tenantedSummary(opts *InstallOptions) string {
	switch opts.TenantedDeploymentMode.Value {
	case sharedTarget.Tenanted:
		return "tenanted deployments only"
	case sharedTarget.TenantedOrUntenanted:
		return "tenanted and untenanted deployments"
	default:
		return "untenanted deployments only"
	}
}

func storageSource(opts *InstallOptions) string {
	if opts.StorageClass.Value != "" {
		return "chosen"
	}
	if len(opts.StorageClasses) == 0 {
		return "no storage classes are readable in this cluster"
	}
	return ""
}

func accessModeSummary(opts *InstallOptions) string {
	if opts.ReadWriteMany.Value {
		return "ReadWriteMany - script pods run on any node"
	}
	return "ReadWriteOnce - script pods run on the agent's node"
}

// accessModeSource names the provisioner the mode was read from, because it is
// worked out rather than asked about.
func accessModeSource(opts *InstallOptions) string {
	if opts.AccessModeChosen {
		return "chosen"
	}

	class, found := opts.resolvedStorageClass()
	switch {
	case !found:
		return "no storage class to read it from"
	case class.SupportsReadWriteMany():
		return fmt.Sprintf("%s serves a shared filesystem", class.Provisioner)
	default:
		return fmt.Sprintf("%s serves one node at a time", class.Provisioner)
	}
}

// permissionsSummary describes the fallback, which is what these values decide.
// What a deployment actually gets can be more than this, whenever a
// WorkloadServiceAccount matches it.
func permissionsSummary(opts *InstallOptions) string {
	switch {
	case opts.RestrictScriptPods.Value:
		return "nothing by default"
	case len(opts.ScriptPodRoles.Value) > 0:
		return "the rules of " + strings.Join(opts.ScriptPodRoles.Value, ", ")
	default:
		return "anything in the cluster"
	}
}

func permissionsSource(opts *InstallOptions) string {
	switch {
	case opts.RestrictScriptPods.Value:
		return "the permissions controller grants each deployment what it needs"
	case len(opts.ScriptPodRoles.Value) > 0:
		return fmt.Sprintf("%d %s copied in now, and not followed afterwards",
			len(opts.ScriptPodRules), octoK8s.Pluralise("rule", "rules", len(opts.ScriptPodRules)))
	case opts.PermissionsController:
		return "the permissions controller can grant less than this per deployment"
	default:
		return "the chart's own default"
	}
}

func credentialPlacement(opts *InstallOptions) string {
	if opts.InlineSecrets.Value {
		return "access token in the Helm values"
	}
	return "access token in a Kubernetes Secret"
}

// RenderReviewForTest prints the review screen without asking anything.
func RenderReviewForTest(opts *InstallOptions) {
	_ = opts.resolveNames()
	shared.PrintReview(opts.Out, reviewGroups(opts))
}
