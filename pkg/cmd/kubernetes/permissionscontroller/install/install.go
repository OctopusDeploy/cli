package install

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ChartRef = helm.ChartRef{Ref: "oci://registry-1.docker.io/octopusdeploy/octopus-permissions-controller-chart"}

// ChartName is the chart's own name, which is how an existing release is
// recognised whatever it was named.
const ChartName = "octopus-permissions-controller-chart"

// Only one controller runs per cluster, so there is nothing to derive a release
// name from.
const DefaultReleaseName = "octopus-permissions-controller"

// MinimumAgentVersion is the first agent release whose script pods ask the
// controller which service account to run as. An older agent ignores it.
const MinimumAgentVersion = "v2.28.1"

// AgentChartRef is printed rather than installed: restricting an agent's script
// pods changes a release this command does not own.
const AgentChartRef = "oci://registry-1.docker.io/octopusdeploy/kubernetes-agent"

const (
	FlagTargetNamespace      = "target-namespace"
	FlagTargetNamespaceRegex = "target-namespace-regex"
	FlagCertManager          = "cert-manager"
	FlagNamespacedRBAC       = "namespaced-rbac"
)

type InstallFlags struct {
	TargetNamespaces     *flag.Flag[[]string]
	TargetNamespaceRegex *flag.Flag[string]
	CertManager          *flag.Flag[bool]
	NamespacedRBAC       *flag.Flag[bool]

	*octoK8s.CommonFlags
}

func NewInstallFlags() *InstallFlags {
	flags := &InstallFlags{
		TargetNamespaces:     flag.New[[]string](FlagTargetNamespace, false),
		TargetNamespaceRegex: flag.New[string](FlagTargetNamespaceRegex, false),
		CertManager:          flag.New[bool](FlagCertManager, false),
		NamespacedRBAC:       flag.New[bool](FlagNamespacedRBAC, false),
		CommonFlags:          octoK8s.NewCommonFlags(),
	}
	// Set here as well as on the cobra flag, because the `kubernetes install`
	// wizard builds these without cobra ever parsing a command line.
	flags.CertManager.Value = true
	return flags
}

type InstallOptions struct {
	*InstallFlags
	*cmd.Dependencies

	// These read the cluster during Discover. Injected so the prompt, review and
	// commit flows can be tested without one.
	CertManagerPresentCallback func() (bool, error)
	ControllerPresentCallback  func() (bool, error)
	ExistingReleasesCallback   func() ([]helm.Release, error)
	AgentsCallback             func() ([]agent.Installation, error)
	NamespacesCallback         func(ctx context.Context) ([]string, error)

	// Populated by Discover before prompting. Exported so tests can drive the
	// prompt flow against a fake cluster.
	Cluster         *octoK8s.Cluster
	Runner          *helm.Runner
	KubeContextInfo octoK8s.Context

	CertManagerPresent bool
	// ControllerPresent is read from the custom resources, which the chart keeps
	// when it is uninstalled, so it can be true with no release behind it.
	ControllerPresent bool
	ExistingRelease   *helm.Release
	Agents            []agent.Installation

	TargetNamespace string
	TargetRelease   string
}

func NewInstallOptions(installFlags *InstallFlags, dependencies *cmd.Dependencies) *InstallOptions {
	opts := &InstallOptions{
		InstallFlags: installFlags,
		Dependencies: dependencies,
	}

	opts.CertManagerPresentCallback = func() (bool, error) { return agent.CertManagerPresent(opts.Cluster) }
	opts.ControllerPresentCallback = func() (bool, error) { return agent.PermissionsControllerPresent(opts.Cluster) }
	opts.ExistingReleasesCallback = func() ([]helm.Release, error) { return opts.Runner.FindByChart(ChartName) }
	opts.AgentsCallback = func() ([]agent.Installation, error) { return agent.Installations(opts.Runner) }
	opts.NamespacesCallback = func(ctx context.Context) ([]string, error) { return listNamespaces(ctx, opts.Cluster) }

	return opts
}

func NewCmdInstall(f factory.Factory) *cobra.Command {
	installFlags := NewInstallFlags()

	command := &cobra.Command{
		Use:   "install",
		Short: "Install the Octopus permissions controller",
		Long: heredoc.Docf(`
			Install the Octopus permissions controller into a Kubernetes cluster.

			The controller decides which service account a Kubernetes agent's script pods run as,
			matching each deployment against the WorkloadServiceAccount resources in the namespace
			it is deploying to. It runs entirely inside the cluster and never contacts Octopus.

			One controller serves the whole cluster. It needs cert-manager for its admission
			webhook's certificate, and Kubernetes agent %s or newer to have any effect.
		`, MinimumAgentVersion),
		Example: heredoc.Docf(`
			$ %[1]s kubernetes permissions-controller install
			$ %[1]s kubernetes permissions-controller install --target-namespace my-app --no-prompt
			$ %[1]s kubernetes permissions-controller install --target-namespace-regex '^team-.*$' --dry-run
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			// The permissions controller never talks to Octopus, so this command
			// works without a login.
			dependencies := &cmd.Dependencies{
				Ask:      f.Ask,
				CmdPath:  c.CommandPath(),
				Out:      c.OutOrStdout(),
				NoPrompt: !f.IsPromptEnabled(),
			}
			return installRun(c.Context(), NewInstallOptions(installFlags, dependencies))
		},
	}

	flags := command.Flags()
	flags.SortFlags = false
	flags.StringArrayVar(&installFlags.TargetNamespaces.Value, FlagTargetNamespace, nil,
		"Namespace the controller manages permissions in. Repeat for more than one. Defaults to every namespace.")
	flags.StringVar(&installFlags.TargetNamespaceRegex.Value, FlagTargetNamespaceRegex, "",
		"Regular expression matched against namespace names, for managing namespaces that do not exist yet.")
	flags.BoolVar(&installFlags.CertManager.Value, FlagCertManager, true,
		"Let cert-manager issue the certificate the controller's mutating admission webhook needs. Turn off only if you are supplying it yourself.")
	flags.BoolVar(&installFlags.NamespacedRBAC.Value, FlagNamespacedRBAC, false,
		"Give the controller permissions in its own namespace only, instead of across the cluster.")
	octoK8s.RegisterCommonFlags(command, installFlags.CommonFlags)
	describeCommonFlags(command)

	return command
}

// describeCommonFlags corrects the shared help for a component that has no name
// to derive anything from, and makes no outbound connection to check.
func describeCommonFlags(command *cobra.Command) {
	flags := command.Flags()
	flags.Lookup(octoK8s.FlagNamespace).Usage = fmt.Sprintf("The namespace to install into. Defaults to %s.", octoK8s.PermissionsControllerNamespace)
	flags.Lookup(octoK8s.FlagReleaseName).Usage = fmt.Sprintf("The Helm release name. Defaults to %s.", DefaultReleaseName)
	flags.Lookup(octoK8s.FlagSkipPreflight).Usage = "Skip the prerequisite checks that run before installing."
	_ = flags.MarkHidden(octoK8s.FlagPreflightImage)
}

// Run installs the controller using an existing set of dependencies. The
// `kubernetes install` wizard uses this to hand off after the user picks a
// component, so the two entry points share one implementation.
func Run(dependencies *cmd.Dependencies) error {
	return installRun(context.Background(), NewInstallOptions(NewInstallFlags(), dependencies))
}

func installRun(ctx context.Context, opts *InstallOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := opts.Discover(ctx); err != nil {
		return err
	}

	if opts.NoPrompt {
		// Nothing is mandatory here: the controller has no name, no credential,
		// and nothing to register with.
		if err := opts.ResolveWithoutPrompting(); err != nil {
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
	connector := &shared.Connector{
		Dependencies:  opts.Dependencies,
		CommonFlags:   opts.CommonFlags,
		SelectMessage: "Which cluster should the permissions controller be installed into?",
		Discover:      opts.discover,
	}

	session, err := connector.Connect(ctx)
	if err != nil {
		return err
	}

	opts.Cluster = session.Cluster
	opts.Runner = session.Runner
	opts.KubeContextInfo = session.Context
	return nil
}

// discover runs inside the connector's retry loop, so it sets what the
// callbacks read from before using them.
func (opts *InstallOptions) discover(_ context.Context, session *shared.Session) error {
	opts.Cluster = session.Cluster
	opts.Runner = session.Runner
	opts.KubeContextInfo = session.Context

	certManager, err := opts.CertManagerPresentCallback()
	if err != nil {
		return err
	}
	opts.CertManagerPresent = certManager

	controller, err := opts.ControllerPresentCallback()
	if err != nil {
		return err
	}
	opts.ControllerPresent = controller

	releases, err := opts.ExistingReleasesCallback()
	if err != nil {
		return err
	}
	if len(releases) > 0 {
		opts.ExistingRelease = &releases[0]
	}

	agents, err := opts.AgentsCallback()
	if err != nil {
		return err
	}
	opts.Agents = agents

	return nil
}

// ResolveWithoutPrompting has only names to work out. Nothing else is
// mandatory: the controller has no name of its own, no credential, and nothing
// to register with. The prerequisite checks in Commit are what stop an install
// that cannot work.
func (opts *InstallOptions) ResolveWithoutPrompting() error {
	opts.resolveNames()
	return nil
}

// resolveNames adopts an existing controller's release, so this install upgrades
// it rather than standing a second one up beside it.
func (opts *InstallOptions) resolveNames() {
	namespace, release := octoK8s.PermissionsControllerNamespace, DefaultReleaseName
	if opts.ExistingRelease != nil {
		namespace, release = opts.ExistingRelease.Namespace, opts.ExistingRelease.Name
	}

	opts.TargetNamespace = shared.OrDefault(opts.Namespace.Value, namespace)
	opts.TargetRelease = shared.OrDefault(opts.ReleaseName.Value, release)
}

func (opts *InstallOptions) chartRef() helm.ChartRef {
	ref := ChartRef
	ref.Version = opts.ChartVersion.Value
	return ref
}

// listNamespaces leaves out the kube-* namespaces, which hold the control plane
// and never run a script pod.
func listNamespaces(ctx context.Context, cluster *octoK8s.Cluster) ([]string, error) {
	list, err := cluster.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list the cluster's namespaces: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		if strings.HasPrefix(item.Name, "kube-") {
			continue
		}
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (opts *InstallOptions) ResolveNamesForTest() {
	opts.resolveNames()
}
