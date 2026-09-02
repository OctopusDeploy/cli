package install

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/argocdgateways"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

const (
	FlagName                   = "name"
	FlagEnvironment            = "environment"
	FlagArgoCDNamespace        = "argocd-namespace"
	FlagArgoCDServerGRPCURL    = "argocd-server-grpc-url"
	FlagArgoCDToken            = "argocd-token"
	FlagArgoCDWebUIURL         = "argocd-web-ui-url"
	FlagOctopusGRPCURL         = "octopus-grpc-url"
	FlagConfigureArgoCDAccount = "configure-argocd-account"
	FlagArgoCDAccountName      = "argocd-account-name"
	FlagAllowSync              = "allow-sync"
	FlagInlineSecrets          = "inline-secrets"
	FlagArgoCDGRPCWeb          = "argocd-grpc-web"
	FlagArgoCDGRPCWebRootPath  = "argocd-grpc-web-root-path"
	FlagArgoCDProjectToken     = "argocd-project-token"
)

type InstallFlags struct {
	Name                   *flag.Flag[string]
	Environments           *flag.Flag[[]string]
	ArgoCDNamespace        *flag.Flag[string]
	ArgoCDServerGRPCURL    *flag.Flag[string]
	ArgoCDToken            *flag.Flag[string]
	ArgoCDWebUIURL         *flag.Flag[string]
	OctopusGRPCURL         *flag.Flag[string]
	ConfigureArgoCDAccount *flag.Flag[bool]
	ArgoCDAccountName      *flag.Flag[string]
	AllowSync              *flag.Flag[bool]
	InlineSecrets          *flag.Flag[bool]
	ArgoCDGRPCWeb          *flag.Flag[bool]
	ArgoCDGRPCWebRootPath  *flag.Flag[string]
	ArgoCDProjectTokens    *flag.Flag[[]string]

	*shared.CommonFlags
}

func NewInstallFlags() *InstallFlags {
	return &InstallFlags{
		Name:                   flag.New[string](FlagName, false),
		Environments:           flag.New[[]string](FlagEnvironment, false),
		ArgoCDNamespace:        flag.New[string](FlagArgoCDNamespace, false),
		ArgoCDServerGRPCURL:    flag.New[string](FlagArgoCDServerGRPCURL, false),
		ArgoCDToken:            flag.New[string](FlagArgoCDToken, true),
		ArgoCDWebUIURL:         flag.New[string](FlagArgoCDWebUIURL, false),
		OctopusGRPCURL:         flag.New[string](FlagOctopusGRPCURL, false),
		ConfigureArgoCDAccount: flag.New[bool](FlagConfigureArgoCDAccount, false),
		ArgoCDAccountName:      flag.New[string](FlagArgoCDAccountName, false),
		AllowSync:              flag.New[bool](FlagAllowSync, false),
		InlineSecrets:          flag.New[bool](FlagInlineSecrets, false),
		ArgoCDGRPCWeb:          flag.New[bool](FlagArgoCDGRPCWeb, false),
		ArgoCDGRPCWebRootPath:  flag.New[string](FlagArgoCDGRPCWebRootPath, false),
		ArgoCDProjectTokens:    flag.New[[]string](FlagArgoCDProjectToken, true),
		CommonFlags:            shared.NewCommonFlags(),
	}
}

type InstallOptions struct {
	*InstallFlags
	*cmd.Dependencies

	GetAllEnvironmentsCallback selectors.GetAllEnvironmentsCallback
	// RegisterCallback registers the gateway with Octopus and returns its own
	// credential. Injected so the flow can be tested without a server.
	RegisterCallback func(argocdgateways.RegisterCommand) (*argocdgateways.Registration, error)
	// DeregisterCallback undoes a registration when the install that followed
	// it did not work.
	DeregisterCallback func(id string) error

	// Populated by Discover before prompting. Exported so tests can drive the
	// prompt flow against a fake cluster.
	Cluster         *octoK8s.Cluster
	Instances       []argocd.Instance
	Instance        argocd.Instance
	Runner          *helm.Runner
	KubeContextInfo octoK8s.Context

	TargetNamespace string
	TargetRelease   string
	Registration    *argocdgateways.Registration
}

func NewInstallOptions(installFlags *InstallFlags, dependencies *cmd.Dependencies) *InstallOptions {
	return &InstallOptions{
		InstallFlags: installFlags,
		Dependencies: dependencies,
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(dependencies.Client)
		},
		RegisterCallback: func(command argocdgateways.RegisterCommand) (*argocdgateways.Registration, error) {
			return argocdgateways.Register(dependencies.Client, command)
		},
		DeregisterCallback: func(id string) error {
			return argocdgateways.DeleteByID(dependencies.Client, dependencies.Space.ID, id)
		},
	}
}

func NewCmdInstall(f factory.Factory) *cobra.Command {
	installFlags := NewInstallFlags()

	command := &cobra.Command{
		Use:   "install",
		Short: "Install the Octopus Argo CD gateway",
		Long: heredoc.Doc(`
			Install the Octopus Argo CD gateway into a Kubernetes cluster.

			The gateway connects an Argo CD instance to Octopus. It runs in the same cluster as
			Argo CD and makes an outgoing connection to Octopus, so Argo CD does not need to be
			reachable from outside the cluster.

			Run without arguments to be prompted. Anything that can be read from the cluster or
			from Octopus is filled in for you: the Argo CD namespace and in-cluster address, how
			Argo CD is serving TLS, the Octopus server and space, and the install namespace.
		`),
		Example: heredoc.Docf(`
			$ %[1]s kubernetes gateway install
			$ %[1]s kubernetes gateway install --name production --environment Production --dry-run
			$ %[1]s kubernetes gateway install --name production --environment Production --argocd-token eyJhbGci... --no-prompt
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewInstallOptions(installFlags, cmd.NewDependencies(f, c))
			return installRun(c.Context(), opts)
		},
	}

	flags := command.Flags()
	flags.SortFlags = false
	flags.StringVarP(&installFlags.Name.Value, FlagName, "n", "", "Name for the Argo CD instance in Octopus. The namespace and Helm release name are derived from it.")
	flags.StringSliceVarP(&installFlags.Environments.Value, FlagEnvironment, "e", nil, "Environment the Argo CD instance serves. Repeat for more than one.")
	flags.StringVar(&installFlags.ArgoCDNamespace.Value, FlagArgoCDNamespace, "", "Namespace Argo CD is installed in. Discovered from the cluster if not set.")
	flags.StringVar(&installFlags.ArgoCDServerGRPCURL.Value, FlagArgoCDServerGRPCURL, "", "In-cluster gRPC URL of the Argo CD API server. Discovered from the cluster if not set.")
	flags.StringVar(&installFlags.ArgoCDToken.Value, FlagArgoCDToken, "", "Argo CD authentication token (JWT) the gateway uses to read from Argo CD.")
	flags.StringVar(&installFlags.ArgoCDWebUIURL.Value, FlagArgoCDWebUIURL, "", "URL of the Argo CD web UI, used for links from Octopus. Discovered from the cluster if not set.")
	flags.StringVar(&installFlags.OctopusGRPCURL.Value, FlagOctopusGRPCURL, "", "gRPC URL of your Octopus Server. Derived from the configured server URL if not set.")
	flags.BoolVar(&installFlags.ConfigureArgoCDAccount.Value, FlagConfigureArgoCDAccount, false, "Create the Argo CD account and RBAC policies Octopus needs, and generate a token.")
	flags.StringVar(&installFlags.ArgoCDAccountName.Value, FlagArgoCDAccountName, argocd.DefaultAccountName, "Name of the Argo CD account Octopus authenticates as.")
	flags.BoolVar(&installFlags.AllowSync.Value, FlagAllowSync, true, "Allow Octopus to sync Argo CD applications, not just read them.")
	flags.BoolVar(&installFlags.InlineSecrets.Value, FlagInlineSecrets, false, "Put credentials directly in the Helm values instead of in Kubernetes Secrets.")
	flags.BoolVar(&installFlags.ArgoCDGRPCWeb.Value, FlagArgoCDGRPCWeb, false, "Tunnel gRPC over HTTP/1.1. Set automatically for AWS managed Argo CD, whose load balancer does not support HTTP/2.")
	flags.StringVar(&installFlags.ArgoCDGRPCWebRootPath.Value, FlagArgoCDGRPCWebRootPath, "", "Root path of the Argo CD API when it is not served at the root, e.g. /argo/api.")
	flags.StringArrayVar(&installFlags.ArgoCDProjectTokens.Value, FlagArgoCDProjectToken, nil, "Argo CD project role token. Repeat per project; the project is read from the token. Required for AWS managed Argo CD, which caps account token lifetimes at 12 hours.")
	shared.RegisterCommonFlags(command, installFlags.CommonFlags, shared.DerivedFromNameDetails())

	return command
}

func installRun(ctx context.Context, opts *InstallOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := opts.Discover(ctx); err != nil {
		return err
	}

	if opts.NoPrompt {
		if err := opts.validateForAutomation(); err != nil {
			return err
		}
		if err := opts.ResolveWithoutPrompting(ctx); err != nil {
			return err
		}
	} else {
		if err := PromptMissing(ctx, opts); err != nil {
			return err
		}
		// Most of this was worked out rather than asked for, so show all of it
		// before anything is created.
		if err := Confirm(ctx, opts); err != nil {
			return err
		}
	}

	return opts.Commit(ctx)
}

func (opts *InstallOptions) Discover(ctx context.Context) error {
	_, err := opts.connector().Connect(ctx)
	return err
}

// connector holds the options' own CommonFlags, so a different cluster chosen
// while retrying is seen by the rest of the install.
func (opts *InstallOptions) connector() *shared.Connector {
	return &shared.Connector{
		Dependencies:  opts.Dependencies,
		CommonFlags:   opts.CommonFlags,
		SelectMessage: "Which cluster should the gateway be installed into?",
		Discover: func(ctx context.Context, session *shared.Session) error {
			opts.Cluster = session.Cluster
			opts.Runner = session.Runner
			opts.KubeContextInfo = session.Context
			return opts.discoverArgoCD(ctx)
		},
		Unrecoverable: func(cause error, kubeConfig *octoK8s.KubeConfig) bool {
			// Nothing to retry, and no other cluster to move to.
			return errors.As(cause, &argocd.ErrNoInstances{}) && len(kubeConfig.Contexts()) == 1
		},
	}
}

// discoverArgoCD covers both hosting models: Argo CD usually runs in the
// cluster, but the EKS capability runs it in the AWS control plane instead,
// where there is nothing in the cluster to find.
func (opts *InstallOptions) discoverArgoCD(ctx context.Context) error {
	// A namespace given explicitly is authoritative: look there directly rather
	// than relying on the labels matching anything expected.
	if opts.ArgoCDNamespace.Value != "" {
		instance, err := argocd.DiscoverInNamespace(ctx, opts.Cluster, opts.ArgoCDNamespace.Value)
		if err != nil {
			return err
		}
		opts.Instances = []argocd.Instance{instance}
		return nil
	}

	instances, err := argocd.Discover(ctx, opts.Cluster)
	if err != nil && !errors.As(err, &argocd.ErrNoInstances{}) {
		return err
	}

	if managed, found, eksErr := argocd.DiscoverEKSManaged(ctx, opts.KubeContextInfo.EKS); found {
		instances = append(instances, managed)
	} else if eksErr != nil {
		// Not fatal: the address can be supplied by flag or prompt instead.
		fmt.Fprintf(opts.Out, "%s Could not read the EKS capabilities for this cluster: %v\n",
			output.Yellow("!"), eksErr)
	}

	opts.Instances = instances

	if len(instances) > 0 {
		return nil
	}

	// An address given by flag need not point at anything discoverable.
	if opts.ArgoCDServerGRPCURL.Value != "" {
		opts.Instances = []argocd.Instance{argocd.NewManagedInstance(opts.ArgoCDServerGRPCURL.Value)}
		return nil
	}

	// The capability may exist without being readable from here.
	if opts.KubeContextInfo.EKS != nil && !opts.NoPrompt {
		return nil
	}

	return argocd.ErrNoInstances{}
}

func (opts *InstallOptions) validateForAutomation() error {
	var missing []string
	if opts.Name.Value == "" {
		missing = append(missing, "--"+FlagName)
	}
	if len(opts.Environments.Value) == 0 {
		missing = append(missing, "--"+FlagEnvironment)
	}
	// A dry run applies nothing, so there is no credential to supply.
	hasCredential := opts.ArgoCDToken.Value != "" ||
		len(opts.ArgoCDProjectTokens.Value) > 0 ||
		opts.ConfigureArgoCDAccount.Value
	if !hasCredential && !opts.DryRun.Value {
		missing = append(missing, fmt.Sprintf("--%s, --%s, or --%s",
			FlagArgoCDToken, FlagArgoCDProjectToken, FlagConfigureArgoCDAccount))
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s must be specified when prompting is disabled", strings.Join(missing, ", "))
	}
	return nil
}

func (opts *InstallOptions) ResolveWithoutPrompting(ctx context.Context) error {
	if err := opts.resolveEnvironments(); err != nil {
		return err
	}

	instance, err := opts.selectInstanceByFlag()
	if err != nil {
		return err
	}
	opts.Instance = instance
	opts.applyInstanceDefaults()

	if err := opts.resolveNames(); err != nil {
		return err
	}

	// The account and RBAC automation edits argocd-cm, which managed Argo CD
	// does not have.
	if opts.Instance.IsManaged() && opts.ConfigureArgoCDAccount.Value {
		return fmt.Errorf("--%s cannot be used with AWS managed Argo CD, which has no in-cluster configuration to edit; "+
			"supply project role tokens with --%s instead", FlagConfigureArgoCDAccount, FlagArgoCDProjectToken)
	}

	if opts.ArgoCDToken.Value == "" && opts.ConfigureArgoCDAccount.Value {
		status, err := argocd.InspectAccount(ctx, opts.Cluster, opts.Instance,
			argocd.AccountSpec{Name: opts.accountName(), AllowSync: opts.AllowSync.Value})
		if err != nil {
			return err
		}

		token, err := ConfigureAccountAndMintToken(ctx, opts, status)
		if err != nil {
			return err
		}
		opts.ArgoCDToken.Value = token
	}

	return nil
}

func (opts *InstallOptions) selectInstanceByFlag() (argocd.Instance, error) {
	if opts.ArgoCDNamespace.Value != "" {
		for _, i := range opts.Instances {
			if i.Namespace == opts.ArgoCDNamespace.Value {
				return i, nil
			}
		}
		return argocd.Instance{}, fmt.Errorf("no Argo CD API server was found in namespace %q", opts.ArgoCDNamespace.Value)
	}

	if len(opts.Instances) > 1 {
		namespaces := make([]string, 0, len(opts.Instances))
		for _, i := range opts.Instances {
			namespaces = append(namespaces, i.Namespace)
		}
		return argocd.Instance{}, fmt.Errorf("this cluster has more than one Argo CD installation (%s), so --%s must be specified",
			strings.Join(namespaces, ", "), FlagArgoCDNamespace)
	}
	return opts.Instances[0], nil
}

func (opts *InstallOptions) applyInstanceDefaults() {
	opts.ArgoCDNamespace.Value = opts.Instance.Namespace
	if opts.ArgoCDServerGRPCURL.Value == "" {
		opts.ArgoCDServerGRPCURL.Value = opts.Instance.ServerGRPCURL
	}
	if opts.ArgoCDWebUIURL.Value == "" {
		opts.ArgoCDWebUIURL.Value = opts.Instance.WebUIURL
	}
	if opts.OctopusGRPCURL.Value == "" {
		opts.OctopusGRPCURL.Value = octoK8s.DeriveGRPCURL(opts.Host)
	}
}

func (opts *InstallOptions) resolveNames() error {
	namespace, release, err := octoK8s.ResolveNames(opts.Namespace.Value, opts.ReleaseName.Value, octoK8s.ArgoCDGatewayNamespacePrefix, opts.Name.Value)
	if err != nil {
		return err
	}
	opts.TargetNamespace, opts.TargetRelease = namespace, release
	return nil
}

// Run installs the gateway using an existing set of dependencies. The
// `kubernetes install` wizard uses this to hand off after the user picks a
// component, so the two entry points share one implementation.
func Run(dependencies *cmd.Dependencies) error {
	return installRun(context.Background(), NewInstallOptions(NewInstallFlags(), dependencies))
}

// resolveEnvironments turns whatever --environment was given into the slug
// Octopus's registration API accepts, matching by name, slug, or ID. The
// prompt already collects slugs, so this only has work to do when the flag was
// used.
func (opts *InstallOptions) resolveEnvironments() error {
	if len(opts.Environments.Value) == 0 || opts.GetAllEnvironmentsCallback == nil {
		return nil
	}

	all, err := opts.GetAllEnvironmentsCallback()
	if err != nil {
		return err
	}

	resolved := make([]string, 0, len(opts.Environments.Value))
	for _, given := range opts.Environments.Value {
		environment, found := matchEnvironment(all, given)
		if !found {
			return fmt.Errorf("no environment named %q exists in this space", given)
		}
		resolved = append(resolved, environmentReference(environment))
	}

	opts.Environments.Value = resolved
	return nil
}

func matchEnvironment(all []*environments.Environment, given string) (*environments.Environment, bool) {
	given = strings.TrimSpace(given)
	for _, environment := range all {
		if strings.EqualFold(environment.Name, given) ||
			strings.EqualFold(environment.Slug, given) ||
			strings.EqualFold(environment.GetID(), given) {
			return environment, true
		}
	}
	return nil, false
}

// accountName is the Argo CD account, or role, Octopus authenticates as.
// Defaulted here as well as during discovery so a caller that reaches a prompt
// by another route cannot end up with a blank one.
func (opts *InstallOptions) accountName() string {
	if opts.ArgoCDAccountName.Value == "" {
		return argocd.DefaultAccountName
	}
	return opts.ArgoCDAccountName.Value
}

func (opts *InstallOptions) useGRPCWeb() bool {
	return opts.ArgoCDGRPCWeb.Value || opts.Instance.GRPCWeb
}

func (opts *InstallOptions) ProjectTokens() ([]argocd.ProjectToken, error) {
	tokens := make([]argocd.ProjectToken, 0, len(opts.ArgoCDProjectTokens.Value))
	seen := map[string]bool{}

	for _, raw := range opts.ArgoCDProjectTokens.Value {
		project, token, err := splitProjectToken(raw)
		if err != nil {
			return nil, err
		}
		if seen[project] {
			return nil, fmt.Errorf("more than one token was given for project %q", project)
		}
		seen[project] = true

		tokens = append(tokens, argocd.ProjectToken{Project: project, Token: token})
	}
	return tokens, nil
}

// splitProjectToken accepts a bare token, reading the project from its subject,
// or an explicit project=token for the rare case of overriding it.
func splitProjectToken(raw string) (project, token string, err error) {
	raw = strings.TrimSpace(raw)

	if before, after, found := strings.Cut(raw, "="); found && !looksLikeToken(raw) {
		project, token = strings.TrimSpace(before), strings.TrimSpace(after)
		if project == "" || token == "" {
			return "", "", fmt.Errorf("--%s must be a token, or project=token", FlagArgoCDProjectToken)
		}
		return project, token, nil
	}

	claims, err := argocd.ParseProjectToken(raw)
	if err != nil {
		return "", "", fmt.Errorf("--%s: %w", FlagArgoCDProjectToken, err)
	}
	return claims.Project, raw, nil
}

func looksLikeToken(value string) bool {
	return strings.Count(value, ".") == 2
}

func (opts *InstallOptions) addProjectToken(token string) error {
	claims, err := argocd.ParseProjectToken(token)
	if err != nil {
		return err
	}
	if claims.Expired() {
		return fmt.Errorf("this token expired on %s", claims.Expires.Format("2 Jan 2006"))
	}

	for _, existing := range opts.ArgoCDProjectTokens.Value {
		if project, _, err := splitProjectToken(existing); err == nil && project == claims.Project {
			return fmt.Errorf("a token for project %q has already been added", claims.Project)
		}
	}

	opts.ArgoCDProjectTokens.Value = append(opts.ArgoCDProjectTokens.Value, token)
	fmt.Fprintf(opts.Out, "  %s Token accepted for project %s, role %s\n",
		output.Green("✔"), output.Cyan(claims.Project), output.Cyan(claims.Role))
	return nil
}
