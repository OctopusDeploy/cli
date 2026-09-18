package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
)

// PromptMissing guards every prompt on its flag, so supplying a flag suppresses
// the matching question and the generated automation command reproduces the run.
func PromptMissing(ctx context.Context, opts *InstallOptions) error {
	opts.resolveNames()

	reportExistingController(opts)
	reportAgents(opts)

	if err := confirmWithoutCertManager(opts); err != nil {
		return err
	}

	return promptForManagedNamespaces(ctx, opts)
}

func reportExistingController(opts *InstallOptions) {
	if opts.ExistingRelease != nil {
		fmt.Fprintf(opts.Out, "\nThe permissions controller is already installed here: release %s in namespace %s.\n",
			output.Cyan(opts.ExistingRelease.Name), output.Cyan(opts.ExistingRelease.Namespace))
		fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
			"Chart %s. This install upgrades it - one controller serves the whole cluster.", opts.ExistingRelease.Version))
		return
	}

	// The chart keeps its custom resources when it is uninstalled, so they
	// outlive the release that created them.
	if opts.ControllerPresent {
		fmt.Fprintf(opts.Out, "\n%s This cluster already serves the controller's custom resources, but no Helm release was found for them.\n",
			output.Yellow("!"))
		fmt.Fprintf(opts.Out, "  %s\n", output.Dim(
			"They are left behind by an uninstalled controller, or by one installed some other way. Installing over them is safe."))
	}
}

// reportAgents says what the controller will have to work with: it only ever
// acts on the script pods an agent creates.
func reportAgents(opts *InstallOptions) {
	if len(opts.Agents) == 0 {
		fmt.Fprintf(opts.Out, "\nNo Kubernetes agent is installed in this cluster, so the controller has nothing to act on yet.\n")
		fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
			"Installing it first is fine - it starts working as soon as an agent %s or newer arrives.", MinimumAgentVersion))
		return
	}

	fmt.Fprintf(opts.Out, "\nKubernetes agents in this cluster:\n")
	for _, installation := range opts.Agents {
		fmt.Fprintf(opts.Out, "  %s %s\n", output.Cyan(installation.Name),
			output.Dimf("(%s in %s)", installation.Mode, installation.Release.Namespace))
	}
}

// confirmWithoutCertManager makes carrying on a deliberate choice: without a
// certificate the webhook installs and then rejects every pod it is asked to
// mutate, which stops deployments rather than merely failing to help them.
func confirmWithoutCertManager(opts *InstallOptions) error {
	if opts.CertManagerPresent || !opts.CertManager.Value {
		return nil
	}

	fmt.Fprintf(opts.Out, "\n%s cert-manager is not installed in this cluster.\n", output.Yellow("!"))
	fmt.Fprintf(opts.Out, "  %s\n", output.Dim(
		"The controller runs a mutating admission webhook, and cert-manager normally issues the certificate it serves with."))

	proceed := false
	if err := opts.Ask(&survey.Confirm{
		Message: "Install anyway, and supply the webhook certificate yourself?",
		Default: false,
		Help:    "Answering no cancels the install, so you can install cert-manager first and start again.",
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("install cancelled")
	}

	opts.CertManager.Value = false
	return nil
}

func promptForManagedNamespaces(ctx context.Context, opts *InstallOptions) error {
	// Either flag already answers this, so asking would override what was asked
	// for.
	if len(opts.TargetNamespaces.Value) > 0 || opts.TargetNamespaceRegex.Value != "" {
		return nil
	}

	const (
		everyNamespace = "Every namespace"
		chosen         = "Only namespaces I choose"
		matching       = "Namespaces matching a pattern"
	)

	answer := ""
	if err := opts.Ask(&survey.Select{
		Message: "Which namespaces should the controller manage permissions in?",
		Options: []string{everyNamespace, chosen, matching},
		Default: everyNamespace,
		Help:    "Outside these namespaces the agent's own default script pod permissions apply, and the controller does nothing.",
	}, &answer); err != nil {
		return err
	}

	switch answer {
	case chosen:
		return promptForNamespaceList(ctx, opts)
	case matching:
		return opts.Ask(&survey.Input{
			Message: "Namespace pattern",
			Help:    "A regular expression matched against namespace names, which also covers namespaces that do not exist yet.",
		}, &opts.TargetNamespaceRegex.Value, survey.WithValidator(survey.Required))
	}
	return nil
}

func promptForNamespaceList(ctx context.Context, opts *InstallOptions) error {
	namespaces, err := opts.NamespacesCallback(ctx)
	if err != nil {
		return err
	}

	selected, err := question.MultiSelectMap(opts.Ask, "Which namespaces?", namespaces,
		func(namespace string) string { return namespace }, true)
	if err != nil {
		return err
	}

	opts.TargetNamespaces.Value = selected
	return nil
}
