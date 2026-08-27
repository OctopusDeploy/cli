package install

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
)

type reviewItem struct {
	Label string
	Value string
	// Source distinguishes a detected value from one that was typed.
	Source string
	// A nil Edit means the value can only be changed by starting again.
	Edit func(context.Context, *InstallOptions) error
}

type reviewGroup struct {
	Title string
	Items []reviewItem
}

// Confirm shows every setting, detected or chosen. Most are worked out rather
// than asked for, which is the point of the wizard, but it also means nobody
// sees them unless they are shown.
func Confirm(ctx context.Context, opts *InstallOptions) error {
	for {
		groups := reviewGroups(opts)
		printReview(opts, groups)

		const (
			install = "Install"
			change  = "Change a setting"
			cancel  = "Cancel"
		)

		answer := ""
		if err := opts.Ask(&survey.Select{
			Message: "Ready to install?",
			Options: []string{install, change, cancel},
		}, &answer); err != nil {
			return err
		}

		switch answer {
		case install:
			return nil
		case cancel:
			return errors.New("install cancelled")
		}

		if err := editSetting(ctx, opts, groups); err != nil {
			return err
		}
	}
}

func editSetting(ctx context.Context, opts *InstallOptions, groups []reviewGroup) error {
	type editable struct {
		label string
		edit  func(context.Context, *InstallOptions) error
	}

	var choices []editable
	for _, group := range groups {
		for _, item := range group.Items {
			if item.Edit != nil {
				choices = append(choices, editable{label: group.Title + ": " + item.Label, edit: item.Edit})
			}
		}
	}

	selected, err := question.SelectMap(opts.Ask, "Which setting?", choices,
		func(e editable) string { return e.label })
	if err != nil {
		return err
	}

	if err := selected.edit(ctx, opts); err != nil {
		return err
	}

	// The namespace and release name follow the instance name unless set
	// explicitly, so they need working out again.
	return opts.resolveNames()
}

func printReview(opts *InstallOptions, groups []reviewGroup) {
	width := 0
	for _, group := range groups {
		for _, item := range group.Items {
			if len(item.Label) > width {
				width = len(item.Label)
			}
		}
	}

	fmt.Fprintf(opts.Out, "\n%s\n", output.Bold("Review the installation"))
	for _, group := range groups {
		fmt.Fprintf(opts.Out, "\n  %s\n", output.Bold(group.Title))
		for _, item := range group.Items {
			fmt.Fprintf(opts.Out, "    %-*s  %s", width, item.Label, output.Cyan(item.Value))
			// An unset value has no source worth claiming.
			if item.Source != "" && !strings.HasPrefix(item.Value, "(") {
				fmt.Fprintf(opts.Out, "  %s", output.Dimf("(%s)", item.Source))
			}
			fmt.Fprintln(opts.Out)
		}
	}
	fmt.Fprintln(opts.Out)
}

func reviewGroups(opts *InstallOptions) []reviewGroup {
	return []reviewGroup{
		{Title: "Cluster", Items: clusterItems(opts)},
		{Title: "Octopus", Items: octopusItems(opts)},
		{Title: "Argo CD", Items: argoItems(opts)},
		{Title: "Helm", Items: helmItems(opts)},
	}
}

func clusterItems(opts *InstallOptions) []reviewItem {
	context := opts.KubeContextInfo
	source := "current context"
	if opts.KubeContext.Value != "" && !context.IsCurrent {
		source = "chosen"
	}

	return []reviewItem{
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
			Source: derivedOrSet(opts.Namespace.Value, "derived from the name"),
			Edit:   editText(&opts.Namespace.Value, "Namespace to install into", func(o *InstallOptions) string { return o.TargetNamespace }),
		},
		{
			Label:  "Helm release",
			Value:  opts.TargetRelease,
			Source: derivedOrSet(opts.ReleaseName.Value, "derived from the name"),
			Edit:   editText(&opts.ReleaseName.Value, "Helm release name", func(o *InstallOptions) string { return o.TargetRelease }),
		},
	}
}

func octopusItems(opts *InstallOptions) []reviewItem {
	return []reviewItem{
		{
			Label: "Name", Value: opts.Name.Value, Source: "chosen",
			Edit: func(_ context.Context, o *InstallOptions) error {
				o.Name.Value = ""
				return question.AskName(o.Ask, "", "Argo CD instance", &o.Name.Value)
			},
		},
		{
			Label: "Environments", Value: strings.Join(opts.Environments.Value, ", "), Source: "chosen",
			Edit: func(_ context.Context, o *InstallOptions) error {
				o.Environments.Value = nil
				return promptForEnvironments(o)
			},
		},
		{Label: "Server", Value: opts.Host, Source: "from your login"},
		{Label: "Registration", Value: "Octopus registers the gateway before installing", Source: "no Octopus credential is stored in the cluster"},
		{Label: "Space", Value: opts.Space.Name, Source: "from your login"},
		{
			Label: "gRPC address", Value: opts.OctopusGRPCURL.Value, Source: "derived from the server address",
			Edit: editText(&opts.OctopusGRPCURL.Value, "Octopus Server gRPC address",
				func(o *InstallOptions) string { return o.OctopusGRPCURL.Value }),
		},
	}
}

func argoItems(opts *InstallOptions) []reviewItem {
	instance := opts.Instance

	items := []reviewItem{
		{Label: "Instance", Value: instance.Display(), Source: instanceSource(instance)},
		{
			Label: "Address", Value: opts.ArgoCDServerGRPCURL.Value, Source: "found in the cluster",
			Edit: editText(&opts.ArgoCDServerGRPCURL.Value, "Argo CD address",
				func(o *InstallOptions) string { return o.ArgoCDServerGRPCURL.Value }),
		},
		{
			Label: "Connection", Value: connectionSummary(opts), Source: "matched to the instance",
			Edit: editConnection,
		},
		{
			Label: "Web UI", Value: orNotSet(opts.ArgoCDWebUIURL.Value), Source: "found in the cluster",
			Edit: editText(&opts.ArgoCDWebUIURL.Value, "Argo CD web UI address (optional)",
				func(o *InstallOptions) string { return o.ArgoCDWebUIURL.Value }),
		},
	}

	if instance.IsManaged() {
		items = append(items, reviewItem{
			Label:  "Project tokens",
			Value:  projectTokenSummary(opts),
			Source: "AWS caps account tokens at 12 hours",
			Edit: func(_ context.Context, o *InstallOptions) error {
				o.ArgoCDProjectTokens.Value = nil
				return promptForProjectTokens(o)
			},
		})
		return items
	}

	return append(items,
		reviewItem{
			Label:  "Account",
			Value:  opts.ArgoCDAccountName.Value,
			Source: accountSource(opts),
		},
		reviewItem{
			Label:  "Token",
			Value:  maskedToken(opts.ArgoCDToken.Value),
			Source: tokenSource(opts),
			Edit: func(_ context.Context, o *InstallOptions) error {
				o.ArgoCDToken.Value = ""
				return askForTokenValue(o)
			},
		},
	)
}

func helmItems(opts *InstallOptions) []reviewItem {
	return []reviewItem{
		{
			Label: "Chart", Value: ChartRef.Ref, Source: "",
		},
		{
			Label: "Chart version", Value: orDefault(opts.ChartVersion.Value, "latest"), Source: "",
			Edit: editText(&opts.ChartVersion.Value, "Chart version (blank for the latest)",
				func(o *InstallOptions) string { return o.ChartVersion.Value }),
		},
		{
			Label: "Credentials", Value: credentialPlacement(opts), Source: "",
			Edit: func(_ context.Context, o *InstallOptions) error {
				return o.Ask(&survey.Confirm{
					Message: "Put credentials directly in the Helm values instead of Kubernetes Secrets?",
					Default: o.InlineSecrets.Value,
					Help:    "Secrets keep credentials out of the Helm release and out of any file written with --output-values.",
				}, &o.InlineSecrets.Value)
			},
		},
		{
			Label: "Timeout", Value: orDefault(opts.Timeout.Value, octoK8s.DefaultTimeout.String()), Source: "",
			Edit: editText(&opts.Timeout.Value, "How long to wait for the release to become ready",
				func(o *InstallOptions) string { return o.Timeout.Value }),
		},
	}
}

func editText(target *string, message string, current func(*InstallOptions) string) func(context.Context, *InstallOptions) error {
	return func(_ context.Context, o *InstallOptions) error {
		value := current(o)
		if err := o.Ask(&survey.Input{Message: message, Default: value}, &value); err != nil {
			return err
		}
		*target = strings.TrimSpace(value)
		return nil
	}
}

// editConnection covers the three settings that are the documented cause of a
// gateway that installs and then never connects.
func editConnection(_ context.Context, o *InstallOptions) error {
	const (
		plaintext = "Argo CD is served without TLS"
		selfSign  = "Argo CD uses a certificate that is not publicly trusted"
		grpcWeb   = "Tunnel gRPC over HTTP/1.1 (needed when a load balancer has no HTTP/2)"
	)

	var current []string
	if o.Instance.Plaintext {
		current = append(current, plaintext)
	}
	if o.Instance.SelfSignedTLS {
		current = append(current, selfSign)
	}
	if o.useGRPCWeb() {
		current = append(current, grpcWeb)
	}

	var chosen []string
	prompt := &survey.MultiSelect{
		Message: "How does the gateway reach Argo CD?",
		Options: []string{plaintext, selfSign, grpcWeb},
		Default: current,
	}
	if err := o.Ask(prompt, &chosen); err != nil {
		return err
	}

	selected := map[string]bool{}
	for _, c := range chosen {
		selected[c] = true
	}

	o.Instance.Plaintext = selected[plaintext]
	o.Instance.SelfSignedTLS = selected[selfSign]
	o.Instance.GRPCWeb = selected[grpcWeb]
	o.ArgoCDGRPCWeb.Value = selected[grpcWeb]
	return nil
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
	if err != nil || len(tokens) == 0 {
		return "(none)"
	}

	projects := make([]string, 0, len(tokens))
	for _, t := range tokens {
		projects = append(projects, t.Project)
	}
	return strings.Join(projects, ", ")
}

func credentialPlacement(opts *InstallOptions) string {
	if opts.InlineSecrets.Value {
		return "Argo CD token in the Helm values"
	}
	return "Argo CD token in a Kubernetes Secret"
}

func maskedToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	return "***"
}

func derivedOrSet(explicit, derivedDescription string) string {
	if explicit != "" {
		return "set"
	}
	return derivedDescription
}

func orNotSet(value string) string {
	return orDefault(value, "(not set)")
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func RenderReviewForDemo(opts *InstallOptions) {
	_ = opts.resolveNames()
	printReview(opts, reviewGroups(opts))
}
