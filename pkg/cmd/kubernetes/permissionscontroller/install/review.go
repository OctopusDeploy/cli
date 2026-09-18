package install

import (
	"context"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	controllerK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/permissionscontroller"
)

func Confirm(ctx context.Context, opts *InstallOptions) error {
	review := &shared.Review{
		Dependencies: opts.Dependencies,
		Groups:       func() []shared.Group { return reviewGroups(opts) },
		Refresh:      func() error { opts.resolveNames(); return nil },
	}
	return review.Confirm(ctx)
}

func reviewGroups(opts *InstallOptions) []shared.Group {
	return []shared.Group{
		{Title: "Cluster", Items: clusterItems(opts)},
		{Title: "Controller", Items: controllerItems(opts)},
		{Title: "Agents", Items: agentItems(opts)},
		{Title: "Helm", Items: helmItems(opts)},
	}
}

func clusterItems(opts *InstallOptions) []shared.Item {
	return shared.ClusterItems(opts.Dependencies, opts.CommonFlags, opts.KubeContextInfo,
		&opts.TargetNamespace, &opts.TargetRelease, nameSource(opts))
}

func nameSource(opts *InstallOptions) string {
	if opts.ExistingRelease != nil {
		return "from the controller already installed"
	}
	return "one controller per cluster"
}

func controllerItems(opts *InstallOptions) []shared.Item {
	items := []shared.Item{
		{
			Label: "Managed namespaces", Value: managedNamespaces(opts), Source: "chosen",
			Edit: func(ctx context.Context) error {
				opts.TargetNamespaces.Value = nil
				opts.TargetNamespaceRegex.Value = ""
				return promptForManagedNamespaces(ctx, opts)
			},
		},
		{
			Label: "Namespace pattern", Value: shared.OrNotSet(opts.TargetNamespaceRegex.Value), Source: "chosen",
			Edit: shared.EditText(opts.Ask, &opts.TargetNamespaceRegex.Value, "Namespace pattern (blank for none)",
				func() string { return opts.TargetNamespaceRegex.Value }),
		},
		{
			Label: "RBAC scope", Value: rbacScope(opts),
			Edit: shared.EditConfirm(opts.Ask, &opts.NamespacedRBAC.Value,
				"Give the controller permissions in its own namespace only?",
				"The controller creates Roles and RoleBindings in the namespaces being deployed to, so restricting it to one namespace only works if that is where everything is deployed."),
		},
		{
			Label: "Webhook certificate", Value: webhookCertificate(opts), Source: certificateSource(opts),
			Edit: shared.EditConfirm(opts.Ask, &opts.CertManager.Value,
				"Let cert-manager issue the webhook's certificate?",
				"Answer no only if you are supplying the certificate yourself. Without one the webhook rejects every pod it is asked to mutate."),
		},
	}

	if opts.ExistingRelease != nil {
		items = append(items, shared.Item{
			Label:  "Already installed",
			Value:  opts.ExistingRelease.Version,
			Source: "this install upgrades it",
		})
	}

	return items
}

// managedNamespaces reports what the controller will act on. An empty list is
// not "none": the chart takes it to mean every namespace.
func managedNamespaces(opts *InstallOptions) string {
	if len(opts.TargetNamespaces.Value) == 0 && opts.TargetNamespaceRegex.Value == "" {
		return "every namespace"
	}
	return shared.OrNone(opts.TargetNamespaces.Value)
}

func rbacScope(opts *InstallOptions) string {
	if opts.NamespacedRBAC.Value {
		return "this namespace only"
	}
	return "cluster-wide"
}

func webhookCertificate(opts *InstallOptions) string {
	if opts.CertManager.Value {
		return "cert-manager"
	}
	return "supplied by you"
}

func certificateSource(opts *InstallOptions) string {
	if opts.CertManagerPresent {
		return "cert-manager found in the cluster"
	}
	return "cert-manager is not installed"
}

func agentItems(opts *InstallOptions) []shared.Item {
	if len(opts.Agents) == 0 {
		return []shared.Item{{
			Label:  "Agents",
			Value:  "none installed in this cluster",
			Source: fmt.Sprintf("the controller does nothing until an agent %s or newer arrives", MinimumAgentVersion),
		}}
	}

	items := make([]shared.Item, 0, len(opts.Agents))
	for _, installation := range opts.Agents {
		items = append(items, shared.Item{
			Label:  installation.Name,
			Value:  fmt.Sprintf("%s in %s", installation.Mode, installation.Release.Namespace),
			Source: scriptPodPermissions(installation),
		})
	}
	return items
}

func scriptPodPermissions(installation agent.Installation) string {
	if installation.ScriptPodClusterRole {
		return "script pods still hold cluster-wide permissions"
	}
	return "script pod permissions already restricted"
}

func helmItems(opts *InstallOptions) []shared.Item {
	return shared.HelmItems(opts.Dependencies, opts.CommonFlags, controllerK8s.ChartRef)
}
