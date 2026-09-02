package install

import (
	"context"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/question"
)

func Confirm(ctx context.Context, opts *InstallOptions) error {
	review := &shared.Review{
		Dependencies: opts.Dependencies,
		Groups:       func() []shared.Group { return reviewGroups(opts) },
		Refresh:      opts.resolveNames,
	}
	return review.Confirm(ctx)
}

func reviewGroups(opts *InstallOptions) []shared.Group {
	return []shared.Group{
		{Title: "Cluster", Items: clusterItems(opts)},
		{Title: "Octopus", Items: octopusItems(opts)},
		{Title: "Argo CD", Items: argoItems(opts)},
		{Title: "Helm", Items: helmItems(opts)},
	}
}

func clusterItems(opts *InstallOptions) []shared.Item {
	context := opts.KubeContextInfo
	source := "current context"
	if opts.KubeContext.Value != "" && !context.IsCurrent {
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
		{Label: "Cluster address", Value: context.Server, Source: "from the kubeconfig"},
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
				return question.AskName(opts.Ask, "", "Argo CD instance", &opts.Name.Value)
			},
		},
		{
			Label: "Environments", Value: strings.Join(opts.Environments.Value, ", "), Source: "chosen",
			Edit: func(context.Context) error {
				opts.Environments.Value = nil
				return promptForEnvironments(opts)
			},
		},
		{Label: "Server", Value: opts.Host, Source: "from your login"},
		{Label: "Registration", Value: "Octopus registers the gateway before installing", Source: "no Octopus credential is stored in the cluster"},
		{Label: "Space", Value: opts.Space.Name, Source: "from your login"},
		{
			Label: "gRPC address", Value: opts.OctopusGRPCURL.Value, Source: "derived from the server address",
			Edit: shared.EditText(opts.Ask, &opts.OctopusGRPCURL.Value, "Octopus Server gRPC address",
				func() string { return opts.OctopusGRPCURL.Value }),
		},
	}
}

func argoItems(opts *InstallOptions) []shared.Item {
	instance := opts.Instance

	items := []shared.Item{
		{Label: "Instance", Value: instance.Display(), Source: instanceSource(instance)},
		{
			Label: "Address", Value: opts.ArgoCDServerGRPCURL.Value, Source: "found in the cluster",
			Edit: shared.EditText(opts.Ask, &opts.ArgoCDServerGRPCURL.Value, "Argo CD address",
				func() string { return opts.ArgoCDServerGRPCURL.Value }),
		},
		{
			Label: "Connection", Value: connectionSummary(opts), Source: "matched to the instance",
			Edit: editConnection(opts),
		},
		{
			Label: "Web UI", Value: shared.OrNotSet(opts.ArgoCDWebUIURL.Value), Source: "found in the cluster",
			Edit: shared.EditText(opts.Ask, &opts.ArgoCDWebUIURL.Value, "Argo CD web UI address (optional)",
				func() string { return opts.ArgoCDWebUIURL.Value }),
		},
	}

	if instance.IsManaged() {
		items = append(items, shared.Item{
			Label:  "Project tokens",
			Value:  projectTokenSummary(opts),
			Source: "AWS caps account tokens at 12 hours",
			Edit: func(context.Context) error {
				opts.ArgoCDProjectTokens.Value = nil
				return promptForProjectTokens(opts)
			},
		})
		return items
	}

	return append(items,
		shared.Item{
			Label:  "Account",
			Value:  opts.ArgoCDAccountName.Value,
			Source: accountSource(opts),
		},
		shared.Item{
			Label:  "Token",
			Value:  shared.Masked(opts.ArgoCDToken.Value),
			Source: tokenSource(opts),
			Edit: func(context.Context) error {
				opts.ArgoCDToken.Value = ""
				return askForTokenValue(opts)
			},
		},
	)
}

func helmItems(opts *InstallOptions) []shared.Item {
	return []shared.Item{
		{
			Label: "Chart", Value: ChartRef.Ref, Source: "",
		},
		{
			Label: "Chart version", Value: shared.OrDefault(opts.ChartVersion.Value, "latest"), Source: "",
			Edit: shared.EditText(opts.Ask, &opts.ChartVersion.Value, "Chart version (blank for the latest)",
				func() string { return opts.ChartVersion.Value }),
		},
		{
			Label: "Credentials", Value: credentialPlacement(opts), Source: "",
			Edit: shared.EditConfirm(opts.Ask, &opts.InlineSecrets.Value,
				"Put credentials directly in the Helm values instead of Kubernetes Secrets?",
				"Secrets keep credentials out of the Helm release and out of any file written with --output-values."),
		},
		{
			Label: "Timeout", Value: shared.OrDefault(opts.Timeout.Value, octoK8s.DefaultTimeout.String()), Source: "",
			Edit: shared.EditText(opts.Ask, &opts.Timeout.Value, "How long to wait for the release to become ready",
				func() string { return opts.Timeout.Value }),
		},
	}
}

// editConnection covers the three settings that are the documented cause of a
// gateway that installs and then never connects.
func editConnection(opts *InstallOptions) func(context.Context) error {
	return func(context.Context) error {
		const (
			plaintext = "Argo CD is served without TLS"
			selfSign  = "Argo CD uses a certificate that is not publicly trusted"
			grpcWeb   = "Tunnel gRPC over HTTP/1.1 (needed when a load balancer has no HTTP/2)"
		)

		var current []string
		if opts.Instance.Plaintext {
			current = append(current, plaintext)
		}
		if opts.Instance.SelfSignedTLS {
			current = append(current, selfSign)
		}
		if opts.useGRPCWeb() {
			current = append(current, grpcWeb)
		}

		var chosen []string
		prompt := &survey.MultiSelect{
			Message: "How does the gateway reach Argo CD?",
			Options: []string{plaintext, selfSign, grpcWeb},
			Default: current,
		}
		if err := opts.Ask(prompt, &chosen); err != nil {
			return err
		}

		selected := map[string]bool{}
		for _, c := range chosen {
			selected[c] = true
		}

		opts.Instance.Plaintext = selected[plaintext]
		opts.Instance.SelfSignedTLS = selected[selfSign]
		opts.Instance.GRPCWeb = selected[grpcWeb]
		opts.ArgoCDGRPCWeb.Value = selected[grpcWeb]
		return nil
	}
}

func connectionSummary(opts *InstallOptions) string {
	var parts []string
	if opts.Instance.Plaintext {
		parts = append(parts, "no TLS")
	} else if opts.Instance.SelfSignedTLS {
		parts = append(parts, "TLS, certificate not verified")
	} else {
		parts = append(parts, "TLS, certificate verified")
	}
	if opts.useGRPCWeb() {
		parts = append(parts, "gRPC-Web")
	}
	return strings.Join(parts, ", ")
}

func instanceSource(instance argocd.Instance) string {
	if instance.IsManaged() {
		return "AWS managed, found through the EKS API"
	}
	return "found in the cluster"
}

func accountSource(opts *InstallOptions) string {
	if opts.ConfigureArgoCDAccount.Value {
		return "created by Octopus"
	}
	return "existing"
}

func tokenSource(opts *InstallOptions) string {
	if opts.ConfigureArgoCDAccount.Value {
		return "generated"
	}
	return "supplied"
}

func projectTokenSummary(opts *InstallOptions) string {
	tokens, err := opts.ProjectTokens()
	if err != nil {
		return "(none)"
	}

	projects := make([]string, 0, len(tokens))
	for _, t := range tokens {
		projects = append(projects, t.Project)
	}
	return shared.OrNone(projects)
}

func credentialPlacement(opts *InstallOptions) string {
	if opts.InlineSecrets.Value {
		return "Argo CD token in the Helm values"
	}
	return "Argo CD token in a Kubernetes Secret"
}

// RenderReviewForTest prints the review screen without asking anything.
func RenderReviewForTest(opts *InstallOptions) {
	_ = opts.resolveNames()
	shared.PrintReview(opts.Out, reviewGroups(opts))
}
