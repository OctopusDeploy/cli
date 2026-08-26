package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

// PromptMissing guards every prompt on its flag, so supplying a flag suppresses
// the matching question and the generated automation command reproduces the run.
func PromptMissing(ctx context.Context, opts *InstallOptions) error {
	// Recorded before discovery fills the rest in, so a supplied flag suppresses
	// its prompt rather than merely seeding it.
	suppliedOctopusGRPCURL := opts.OctopusGRPCURL.Value != ""

	if err := question.AskName(opts.Ask, "", "Argo CD instance", &opts.Name.Value); err != nil {
		return err
	}

	if err := opts.resolveNames(); err != nil {
		return err
	}

	if err := promptForEnvironments(opts); err != nil {
		return err
	}

	if err := promptForInstance(opts); err != nil {
		return err
	}
	opts.applyInstanceDefaults()

	if !suppliedOctopusGRPCURL {
		if err := promptForOctopusGRPCURL(opts); err != nil {
			return err
		}
	}

	return promptForArgoCDToken(ctx, opts)
}

func promptForEnvironments(opts *InstallOptions) error {
	if len(opts.Environments.Value) > 0 {
		return nil
	}

	selected, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.GetAllEnvironmentsCallback,
		"Which environments does this Argo CD instance serve?", true)
	if err != nil {
		return err
	}

	for _, e := range selected {
		opts.Environments.Value = append(opts.Environments.Value, environmentReference(e))
	}
	return nil
}

func environmentReference(e *environments.Environment) string {
	if e.Slug != "" {
		return e.Slug
	}
	return e.Name
}

// promptForInstance usually has nothing to ask: discovery normally finds
// exactly one instance.
func promptForInstance(opts *InstallOptions) error {
	if opts.ArgoCDNamespace.Value != "" {
		instance, err := opts.selectInstanceByFlag()
		if err != nil {
			return err
		}
		opts.Instance = instance
		return nil
	}

	// An unreadable EKS capability still has an Argo CD behind it.
	if len(opts.Instances) == 0 {
		return promptForManagedEndpoint(opts)
	}

	instance, err := selectors.Select(opts.Ask, "Which Argo CD instance should the gateway connect to?",
		func() ([]argocd.Instance, error) { return opts.Instances, nil },
		func(i argocd.Instance) string { return i.Display() })
	if err != nil {
		return err
	}
	opts.Instance = instance

	if instance.IsManaged() {
		fmt.Fprintf(opts.Out, "AWS managed Argo CD at %s\n", output.Cyan(instance.ServerGRPCURL))
		fmt.Fprintf(opts.Out, "  %s\n", output.Dim("(TLS verified, gRPC tunnelled over HTTP/1.1 - AWS's load balancer does not support HTTP/2)"))
		if instance.Status != "" && !strings.EqualFold(instance.Status, "ACTIVE") {
			fmt.Fprintf(opts.Out, "  %s The capability is %s, so the gateway may not be able to connect yet.\n",
				output.Yellow("!"), instance.Status)
		}
		return nil
	}

	fmt.Fprintf(opts.Out, "Argo CD %s in namespace %s\n", output.Cyan(instance.Version), output.Cyan(instance.Namespace))
	fmt.Fprintf(opts.Out, "  in-cluster address %s %s\n", output.Cyan(instance.ServerGRPCURL), output.Dim(tlsDescription(instance)))
	return nil
}

// tlsDescription is shown rather than decided silently: getting these wrong is
// a documented cause of a gateway that installs but never connects.
// promptForManagedEndpoint asks for the address of an Argo CD that is not
// running in this cluster, which is how the EKS capability for Argo CD works.
func promptForManagedEndpoint(opts *InstallOptions) error {
	fmt.Fprintf(opts.Out, "\nNo Argo CD is running in this cluster.\n")
	if opts.KubeContextInfo.EKS != nil {
		fmt.Fprintf(opts.Out, "  %s\n", output.Dim(
			"This is an EKS cluster. If you are using the EKS capability for Argo CD, AWS runs Argo CD in its own "+
				"control plane and gives it a public address, which you can find on the cluster's Capabilities tab."))
	}

	endpoint := ""
	if err := opts.Ask(&survey.Input{
		Message: "Argo CD API address",
		Help:    "For AWS managed Argo CD this looks like xxxxxxxx.eks-capabilities.<region>.amazonaws.com.",
	}, &endpoint, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	opts.Instance = argocd.NewManagedInstance(endpoint)
	return nil
}

func tlsDescription(instance argocd.Instance) string {
	switch {
	case instance.Plaintext:
		return "(Argo CD is running in insecure mode, so TLS will be disabled on this connection)"
	case instance.SelfSignedTLS:
		return "(TLS on, certificate verification off - Argo CD's default certificate is self-signed)"
	default:
		return "(TLS on)"
	}
}

func promptForOctopusGRPCURL(opts *InstallOptions) error {
	// Derived from the URL the CLI is logged in to, which is nearly always
	// right - but a proxy forwarding only HTTPS breaks the gateway, so confirm.
	return opts.Ask(&survey.Input{
		Message: "Octopus Server gRPC address",
		Default: opts.OctopusGRPCURL.Value,
		Help: "The gateway holds a gRPC connection to Octopus on port 8443, separate from the REST API on 443. " +
			"If Octopus sits behind a load balancer or proxy, port 8443 must be forwarded to it.",
	}, &opts.OctopusGRPCURL.Value, survey.WithValidator(survey.Required))
}

// promptForArgoCDToken offers to do the whole thing, because creating the token
// by hand means editing two ConfigMaps and running the Argo CD CLI.
func promptForArgoCDToken(ctx context.Context, opts *InstallOptions) error {
	if opts.ArgoCDToken.Value != "" {
		return nil
	}

	// A dry run never reaches Argo CD, so there is no need to find a token.
	if opts.DryRun.Value {
		return nil
	}

	// Managed Argo CD has no argocd-cm to edit, and authenticates with project
	// role tokens because AWS caps account tokens at 12 hours.
	if opts.Instance.IsManaged() {
		return promptForProjectTokens(opts)
	}

	spec := argocd.AccountSpec{Name: opts.accountName(), AllowSync: opts.AllowSync.Value}
	status, err := argocd.InspectAccount(ctx, opts.Cluster, opts.Instance, spec)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "\n%s\n", status.Summary())

	if !opts.ConfigureArgoCDAccount.Value {
		if err := askToConfigureAccount(opts, status); err != nil {
			return err
		}
	}

	if !opts.ConfigureArgoCDAccount.Value {
		printManualTokenInstructions(opts, status)
		return askForTokenValue(opts)
	}

	token, err := ConfigureAccountAndMintToken(ctx, opts, status)
	if err != nil {
		fmt.Fprintf(opts.Out, "\n%s Could not set Argo CD up automatically: %v\n", output.Yellow("!"), err)
		opts.ConfigureArgoCDAccount.Value = false
		printManualTokenInstructions(opts, status)
		return askForTokenValue(opts)
	}

	opts.ArgoCDToken.Value = token
	return nil
}

func askToConfigureAccount(opts *InstallOptions, status argocd.AccountStatus) error {
	message := fmt.Sprintf("Generate an Argo CD token for the %q account?", status.Spec.Name)
	help := "Octopus needs an Argo CD token to read applications and clusters."

	if !status.IsComplete() {
		fmt.Fprintf(opts.Out, "\nThese changes would be made to your Argo CD configuration:\n%s",
			argocd.AccountPatchPlan(opts.Instance.Namespace, status))
		message = "Apply these changes and generate a token?"
		help = "Existing accounts and RBAC rules are left alone; only the entries shown above are added."
	}

	return opts.Ask(&survey.Confirm{Message: message, Default: true, Help: help}, &opts.ConfigureArgoCDAccount.Value)
}

func askForTokenValue(opts *InstallOptions) error {
	return opts.Ask(&survey.Password{
		Message: "Argo CD authentication token",
		Help:    "A JWT for an Argo CD account that can read applications, clusters and logs.",
	}, &opts.ArgoCDToken.Value, survey.WithValidator(survey.Required))
}

// printManualTokenInstructions makes declining the automation a real choice
// rather than a dead end.
func printManualTokenInstructions(opts *InstallOptions, status argocd.AccountStatus) {
	if status.IsComplete() {
		fmt.Fprintf(opts.Out, "\nGenerate a token with:\n  %s\n\n",
			output.Cyan(fmt.Sprintf("argocd account generate-token --account %s", status.Spec.Name)))
		return
	}

	namespace := opts.Instance.Namespace
	fmt.Fprintf(opts.Out, "\nTo set this up by hand:\n")
	if !status.HasAPIKeyCapability || status.Disabled {
		fmt.Fprintf(opts.Out, "  %s\n", output.Cyan(fmt.Sprintf(
			"kubectl patch cm %s -n %s --type merge -p '{\"data\":{\"accounts.%s\":\"apiKey\",\"accounts.%s.enabled\":\"true\"}}'",
			argocd.ConfigMapName, namespace, status.Spec.Name, status.Spec.Name)))
	}
	if len(status.MissingPolicies) > 0 {
		fmt.Fprintf(opts.Out, "  %s\n", output.Dim(fmt.Sprintf(
			"# add these lines to policy.csv in the %s/%s ConfigMap:", namespace, argocd.RBACConfigMapName)))
		for _, p := range status.MissingPolicies {
			fmt.Fprintf(opts.Out, "  %s\n", output.Cyan("  "+p))
		}
	}
	fmt.Fprintf(opts.Out, "  %s\n\n", output.Cyan(fmt.Sprintf("argocd account generate-token --account %s", status.Spec.Name)))
}

func promptForProjectTokens(opts *InstallOptions) error {
	if len(opts.ArgoCDProjectTokens.Value) > 0 {
		return nil
	}

	projects, err := prepareProjectRoles(opts)
	if err != nil {
		return err
	}

	printProjectTokenPreamble(opts)

	if len(projects) == 0 {
		return promptForUnknownProjectTokens(opts)
	}

	for _, project := range projects {
		if err := promptForProjectToken(opts, project); err != nil {
			return err
		}
	}
	return nil
}

// promptForProjectToken links straight at the role that needs the token, then
// takes it.
func promptForProjectToken(opts *InstallOptions, project string) error {
	role := opts.accountName()

	fmt.Fprintf(opts.Out, "\n%s\n", output.Bold("Project "+project))
	if url := argocd.ProjectRolePageURL(opts.Instance.WebUIURL, project, role); url != "" {
		fmt.Fprintf(opts.Out, "  %s\n", output.Blue(url))
	}
	fmt.Fprintf(opts.Out, "  %s\n",
		output.Dimf("or: argocd proj role create-token %s %s", project, role))

	for {
		token := ""
		if err := opts.Ask(&survey.Password{
			Message: fmt.Sprintf("Token for project %s", project),
		}, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		claims, err := argocd.ParseProjectToken(token)
		switch {
		case err != nil:
			fmt.Fprintf(opts.Out, "  %s %v\n", output.Red("✘"), err)
			continue
		case claims.Project != project:
			fmt.Fprintf(opts.Out, "  %s That token is for project %s, not %s.\n",
				output.Red("✘"), output.Cyan(claims.Project), output.Cyan(project))
			continue
		case claims.Expired():
			fmt.Fprintf(opts.Out, "  %s That token expired on %s.\n",
				output.Red("✘"), claims.Expires.Format("2 Jan 2006"))
			continue
		}

		opts.ArgoCDProjectTokens.Value = append(opts.ArgoCDProjectTokens.Value, token)
		return nil
	}
}

// promptForUnknownProjectTokens is the fallback for an Argo CD whose projects
// could not be read, where the project can only come from the token itself.
func promptForUnknownProjectTokens(opts *InstallOptions) error {
	fmt.Fprintf(opts.Out, "  %s\n\n", output.Dimf(
		"argocd proj role create-token <project> %s", opts.ArgoCDAccountName.Value))

	for {
		token := ""
		if err := opts.Ask(&survey.Password{
			Message: "Argo CD project role token",
			Help:    "Which project it belongs to is read from the token itself.",
		}, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		if err := opts.addProjectToken(token); err != nil {
			fmt.Fprintf(opts.Out, "  %s %v\n", output.Red("✘"), err)
			continue
		}

		another := false
		if err := opts.Ask(&survey.Confirm{
			Message: "Add a token for another project?",
			Default: false,
		}, &another); err != nil {
			return err
		}
		if !another {
			return nil
		}
	}
}

// prepareProjectRoles creates the role Octopus authenticates as on the chosen
// projects, and reports which they are. AWS signs the tokens themselves, but
// the role and its policies live in the AppProject in the cluster.
func prepareProjectRoles(opts *InstallOptions) ([]string, error) {
	ctx := context.Background()

	projects, err := argocd.ListProjects(ctx, opts.Cluster, opts.Instance.Namespace)
	if err != nil || len(projects) == 0 {
		// Without the projects there is nothing to offer; the tokens can still
		// be pasted in.
		return nil, err
	}

	selected, err := question.MultiSelectMap(opts.Ask,
		"Which Argo CD projects should Octopus see?", projects,
		func(p argocd.Project) string { return p.Display() }, true)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(selected))
	statuses := make([]argocd.ProjectRoleStatus, 0, len(selected))
	for _, project := range selected {
		names = append(names, project.Name)

		status, err := argocd.InspectProjectRole(ctx, opts.Cluster, opts.Instance.Namespace,
			argocd.ProjectRoleSpec{
				Project:   project.Name,
				Role:      opts.accountName(),
				AllowSync: opts.AllowSync.Value,
			})
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}

	plan := argocd.ProjectRolePatchPlan(opts.Instance.Namespace, statuses)
	if plan == "" {
		fmt.Fprintf(opts.Out, "\nEvery chosen project already grants Octopus what it needs.\n")
		return names, nil
	}

	fmt.Fprintf(opts.Out, "\nThese changes would be made to your Argo CD projects:\n%s", plan)

	proceed := false
	if err := opts.Ask(&survey.Confirm{
		Message: "Apply these?",
		Default: true,
		Help:    "Other roles on these projects are left alone.",
	}, &proceed); err != nil {
		return nil, err
	}
	if !proceed {
		return names, nil
	}

	for _, status := range statuses {
		if err := argocd.EnsureProjectRole(ctx, opts.Cluster, opts.Instance.Namespace, status); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(opts.Out, "%s Argo CD projects updated\n", output.Green("✔"))
	return names, nil
}

func printProjectTokenPreamble(opts *InstallOptions) {
	fmt.Fprintf(opts.Out, "\nAWS signs Argo CD tokens in its own control plane, so Octopus cannot generate\n"+
		"these for you. Each one is a project role token, because AWS caps account\n"+
		"tokens at 12 hours.\n")
}

// PromptForProjectTokenForTest exposes the per-project prompt so its output can
// be asserted on.
func PromptForProjectTokenForTest(opts *InstallOptions, project string) error {
	return promptForProjectToken(opts, project)
}
