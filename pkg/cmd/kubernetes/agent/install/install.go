package install

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/accesstokens"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/permissionscontroller"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/octopusservernodes"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workers"
	"github.com/spf13/cobra"
)

const (
	FlagName               = "name"
	FlagServerCommsAddress = "server-comms-address"
	FlagServerCertificate  = "server-certificate"
	FlagDefaultNamespace   = "default-namespace"
	FlagStorageClass       = "storage-class"
	FlagReadWriteMany      = "read-write-many"
	FlagAcceptEula         = "accept-eula"
	FlagInlineSecrets      = "inline-secrets"
	FlagRestrictScriptPods = "restrict-script-pod-permissions"
	FlagScriptPodRole      = "script-pod-role"
	FlagKubernetesMonitor  = "kubernetes-monitor"
)

// The agent registers itself, so an Octopus credential has to reach the
// cluster. Passing it by Secret reference keeps it out of the Helm release
// values, out of any file written by --output-values, and out of the process
// table. The key name is the chart's contract.
const (
	tokenSecretName = "octopus-agent-registration-token"
	tokenSecretKey  = "bearer-token"
)

// eulaURL is shown rather than assumed: the chart will not install without
// acceptEula, and nobody should be accepting an agreement they were not offered.
const eulaURL = "https://octopus.com/company/legal"

type InstallFlags struct {
	Name                 *flag.Flag[string]
	ServerCommsAddresses *flag.Flag[[]string]
	ServerCertificate    *flag.Flag[string]
	DefaultNamespace     *flag.Flag[string]
	StorageClass         *flag.Flag[string]
	ReadWriteMany        *flag.Flag[bool]
	AcceptEula           *flag.Flag[bool]
	InlineSecrets        *flag.Flag[bool]
	RestrictScriptPods   *flag.Flag[bool]
	ScriptPodRoles       *flag.Flag[[]string]
	KubernetesMonitor    *flag.Flag[bool]

	*sharedTarget.CreateTargetEnvironmentFlags
	*sharedTarget.CreateTargetRoleFlags
	*sharedTarget.CreateTargetTenantFlags
	*machinescommon.CreateTargetMachinePolicyFlags
	*sharedWorker.WorkerPoolFlags
	*shared.CommonFlags
}

func NewInstallFlags() *InstallFlags {
	return &InstallFlags{
		Name:                 flag.New[string](FlagName, false),
		ServerCommsAddresses: flag.New[[]string](FlagServerCommsAddress, false),
		ServerCertificate:    flag.New[string](FlagServerCertificate, false),
		DefaultNamespace:     flag.New[string](FlagDefaultNamespace, false),
		StorageClass:         flag.New[string](FlagStorageClass, false),
		ReadWriteMany:        flag.New[bool](FlagReadWriteMany, false),
		AcceptEula:           flag.New[bool](FlagAcceptEula, false),
		InlineSecrets:        flag.New[bool](FlagInlineSecrets, false),
		RestrictScriptPods:   flag.New[bool](FlagRestrictScriptPods, false),
		ScriptPodRoles:       flag.New[[]string](FlagScriptPodRole, false),
		KubernetesMonitor:    flag.New[bool](FlagKubernetesMonitor, false),

		CreateTargetEnvironmentFlags:   sharedTarget.NewCreateTargetEnvironmentFlags(),
		CreateTargetRoleFlags:          sharedTarget.NewCreateTargetRoleFlags(),
		CreateTargetTenantFlags:        sharedTarget.NewCreateTargetTenantFlags(),
		CreateTargetMachinePolicyFlags: machinescommon.NewCreateTargetMachinePolicyFlags(),
		WorkerPoolFlags:                sharedWorker.NewWorkerPoolFlags(),
		CommonFlags:                    shared.NewCommonFlags(),
	}
}

type InstallOptions struct {
	*InstallFlags
	*cmd.Dependencies

	// Mode decides what the agent registers as, which questions are asked, and
	// which half of the chart's values are filled in. One agent is either a
	// deployment target or a worker; the chart registers it one way or the other.
	Mode agentK8s.Mode

	*sharedTarget.CreateTargetEnvironmentOptions
	*machinescommon.CreateTargetMachinePolicyOptions
	*sharedWorker.WorkerPoolOptions

	// AccessTokenCallback gets the credential the agent registers itself with.
	// Injected so the flow can be tested without a server.
	AccessTokenCallback func() (accesstokens.Token, error)
	// RegisteredCallback reports whether Octopus already has a deployment target
	// or worker of this name, which is how a clash is caught before installing
	// and an orphan is caught after a failure.
	RegisteredCallback func(name string) (bool, error)
	// TargetTagsCallback lists the target tags the space already knows about.
	TargetTagsCallback func() ([]string, error)
	// ServerNodesCallback lists the Octopus Server's own task-running nodes,
	// which is how a High Availability cluster is recognised: the agent polls
	// every node, and each needs its own address.
	ServerNodesCallback func() ([]octopusservernodes.Node, error)

	// Populated by Discover before prompting. Exported so tests can drive the
	// prompt flow against a fake cluster.
	Cluster           *octoK8s.Cluster
	Runner            *helm.Runner
	KubeContextInfo   octoK8s.Context
	StorageClasses    []octoK8s.StorageClass
	NodeArchitectures []string
	// KnownTargetTags is what the space already had, so the review can say which
	// of the chosen tags are new.
	KnownTargetTags       []string
	Installations         []agentK8s.Installation
	PermissionsController bool
	// ScriptPodRules are the rules copied from ScriptPodRole, in the plain shape
	// a Helm value has to be.
	ScriptPodRules []any

	// AccessModeChosen records that --read-write-many was given explicitly, so
	// the storage class does not get to decide.
	AccessModeChosen bool

	TargetNamespace string
	TargetRelease   string
	Token           accesstokens.Token

	// registeredBefore is the answer to RegisteredCallback for
	// registrationCheckedFor, kept so the same question is not asked of Octopus
	// three times in one run.
	registeredBefore       bool
	registrationCheckedFor string

	// serverNodes is the answer from ServerNodesCallback, read once and kept.
	serverNodes     []octopusservernodes.Node
	serverNodesRead bool
}

// alreadyRegistered answers whether Octopus already has an agent of this name.
// It is asked before installing, to warn that a name is taken, and again after
// a failure, to tell an orphaned registration from one that was always there.
func (opts *InstallOptions) alreadyRegistered() (bool, error) {
	if opts.registrationCheckedFor == opts.Name.Value {
		return opts.registeredBefore, nil
	}
	if opts.RegisteredCallback == nil {
		return false, nil
	}

	taken, err := opts.RegisteredCallback(opts.Name.Value)
	if err != nil {
		return false, err
	}
	opts.registeredBefore = taken
	opts.registrationCheckedFor = opts.Name.Value
	return taken, nil
}

func NewInstallOptions(installFlags *InstallFlags, dependencies *cmd.Dependencies, mode agentK8s.Mode) *InstallOptions {
	return &InstallOptions{
		InstallFlags: installFlags,
		Dependencies: dependencies,
		Mode:         mode,

		CreateTargetEnvironmentOptions:   sharedTarget.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetMachinePolicyOptions: machinescommon.NewCreateTargetMachinePolicyOptions(dependencies),
		WorkerPoolOptions:                sharedWorker.NewWorkerPoolOptions(dependencies),

		AccessTokenCallback: func() (accesstokens.Token, error) {
			return accesstokens.Generate(dependencies.Client)
		},
		RegisteredCallback: func(name string) (bool, error) {
			return registered(dependencies, mode, name)
		},
		TargetTagsCallback: func() ([]string, error) {
			return sharedTarget.TargetTagNames(dependencies.Client)
		},
		ServerNodesCallback: func() ([]octopusservernodes.Node, error) {
			return octopusservernodes.TaskNodes(dependencies.Client)
		},
	}
}

func NewCmdInstall(f factory.Factory) *cobra.Command {
	return newCmdInstall(f, agentK8s.ModeDeploymentTarget)
}

func NewCmdWorkerInstall(f factory.Factory) *cobra.Command {
	return newCmdInstall(f, agentK8s.ModeWorker)
}

func newCmdInstall(f factory.Factory, mode agentK8s.Mode) *cobra.Command {
	installFlags := NewInstallFlags()

	command := &cobra.Command{
		Use:     "install",
		Short:   shortDescription(mode),
		Long:    longDescription(mode),
		Example: examples(mode),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewInstallOptions(installFlags, cmd.NewDependencies(f, c), mode)
			opts.AccessModeChosen = c.Flags().Changed(FlagReadWriteMany)
			return installRun(c.Context(), opts)
		},
	}

	flags := command.Flags()
	flags.SortFlags = false
	flags.StringVarP(&installFlags.Name.Value, FlagName, "n", "", fmt.Sprintf(
		"Name for the %s in Octopus. The namespace and Helm release name are derived from it.", mode))
	registerModeFlags(command, installFlags, mode)
	flags.StringVar(&installFlags.MachinePolicy.Value, machinescommon.FlagMachinePolicy, "", fmt.Sprintf(
		"Machine policy the %s is registered with. Uses the default machine policy if not set.", mode))
	flags.StringArrayVar(&installFlags.ServerCommsAddresses.Value, FlagServerCommsAddress, nil,
		"Polling address of your Octopus Server. Derived from the configured server URL if not set. For a High Availability cluster, repeat for each node - the agent polls every node, and each needs its own address.")
	flags.StringVar(&installFlags.ServerCertificate.Value, FlagServerCertificate, "", "Base64-encoded PEM certificate to trust when Octopus is not served by a publicly trusted certificate.")
	flags.StringVar(&installFlags.StorageClass.Value, FlagStorageClass, "", "Storage class for the agent's volume. Uses the cluster's default storage class if not set.")
	flags.BoolVar(&installFlags.ReadWriteMany.Value, FlagReadWriteMany, false, "Request a ReadWriteMany volume, so script pods can run on any node. Read from the storage class if not set.")
	flags.BoolVar(&installFlags.AcceptEula.Value, FlagAcceptEula, false, "Accept the Octopus Customer Agreement ("+eulaURL+"). Required to install.")
	flags.BoolVar(&installFlags.InlineSecrets.Value, FlagInlineSecrets, false, "Put the registration credential directly in the Helm values instead of in a Kubernetes Secret.")
	flags.BoolVar(&installFlags.RestrictScriptPods.Value, FlagRestrictScriptPods, false, "Give script pods no permissions of their own, leaving every deployment to the Octopus permissions controller.")
	flags.StringArrayVar(&installFlags.ScriptPodRoles.Value, FlagScriptPodRole, nil,
		"Give script pods the rules of this role, copied in at install time. Name a cluster role, or a role in a namespace as namespace/name. Repeat for more than one.")
	shared.RegisterCommonFlags(command, installFlags.CommonFlags, shared.DerivedFromNameDetails())

	return command
}

// registerModeFlags keeps a worker from advertising deployment target settings
// it cannot use, and the other way round.
func registerModeFlags(command *cobra.Command, installFlags *InstallFlags, mode agentK8s.Mode) {
	if mode == agentK8s.ModeWorker {
		sharedWorker.RegisterCreateWorkerWorkerPoolFlags(command, installFlags.WorkerPoolFlags)
		return
	}

	sharedTarget.RegisterCreateTargetEnvironmentFlags(command, installFlags.CreateTargetEnvironmentFlags)
	registerTargetTagFlags(command, installFlags)
	sharedTarget.RegisterCreateTargetTenantFlags(command, installFlags.CreateTargetTenantFlags)
	command.Flags().StringVar(&installFlags.DefaultNamespace.Value, FlagDefaultNamespace, "",
		"Namespace deployments go to when the step or the manifest does not name one.")
	// The monitor watches deployed objects, which only a deployment target has.
	command.Flags().BoolVar(&installFlags.KubernetesMonitor.Value, FlagKubernetesMonitor, false,
		"Also install the Kubernetes monitor, which streams live status of the deployed objects back to Octopus. Needs an Octopus Server that supports it.")
}

// registerTargetTagFlags describes these as target tags rather than roles.
// Octopus renamed them, and a tag that does not exist yet is a normal thing to
// give an agent: Octopus creates it when the agent registers.
func registerTargetTagFlags(command *cobra.Command, installFlags *InstallFlags) {
	flags := command.Flags()
	flags.StringSliceVar(&installFlags.Roles.Value, sharedTarget.FlagRole, nil,
		"Target tag for the deployment target. Repeat for more than one. A tag that does not exist yet is created when the agent registers.")
	flags.StringSliceVar(&installFlags.Tags.Value, sharedTarget.FlagTag, nil,
		"Target tag in canonical TagSetName/TagName form, checked against the tag sets. Repeat for more than one.")
}

func installRun(ctx context.Context, opts *InstallOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := opts.Discover(ctx); err != nil {
		return err
	}

	if opts.NoPrompt {
		if err := opts.ValidateForAutomation(); err != nil {
			return err
		}
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

// Run installs using an existing set of dependencies. The `kubernetes install`
// wizard uses this to hand off after the user picks a component, so the two
// entry points share one implementation.
func Run(dependencies *cmd.Dependencies) error {
	return installRun(context.Background(), NewInstallOptions(NewInstallFlags(), dependencies, agentK8s.ModeDeploymentTarget))
}

func RunWorker(dependencies *cmd.Dependencies) error {
	return installRun(context.Background(), NewInstallOptions(NewInstallFlags(), dependencies, agentK8s.ModeWorker))
}

func (opts *InstallOptions) Discover(ctx context.Context) error {
	connector := opts.connector()
	session, err := connector.Connect(ctx)
	if err != nil {
		return err
	}

	opts.Cluster = session.Cluster
	opts.Runner = session.Runner
	opts.KubeContextInfo = session.Context
	return nil
}

func (opts *InstallOptions) connector() *shared.Connector {
	return &shared.Connector{
		Dependencies:  opts.Dependencies,
		CommonFlags:   opts.CommonFlags,
		SelectMessage: fmt.Sprintf("Which cluster should the %s be installed into?", opts.installedThing()),
		Discover:      opts.discoverCluster,
		Unrecoverable: func(cause error, kubeConfig *octoK8s.KubeConfig) bool {
			// Nothing to retry, and no other cluster to move to.
			return errors.As(cause, &agentK8s.ErrUnsupportedNodes{}) && len(kubeConfig.Contexts()) == 1
		},
	}
}

// discoverCluster reads everything the questions and the review screen are
// filled in from, so a credential problem anywhere in it can be fixed and
// retried as one unit.
func (opts *InstallOptions) discoverCluster(ctx context.Context, session *shared.Session) error {
	architectures, err := session.Cluster.NodeArchitectures(ctx)
	if err != nil {
		return err
	}
	opts.NodeArchitectures = architectures

	if !agentK8s.RunnableArchitecture(architectures) {
		return agentK8s.ErrUnsupportedNodes{Architectures: architectures}
	}

	classes, err := session.Cluster.StorageClasses(ctx)
	if err != nil {
		return err
	}
	opts.StorageClasses = classes

	installations, err := agentK8s.Installations(session.Runner)
	if err != nil {
		return err
	}
	opts.Installations = installations

	present, err := permissionscontroller.Present(session.Cluster)
	if err != nil {
		return err
	}
	opts.PermissionsController = present

	return nil
}

func (opts *InstallOptions) ValidateForAutomation() error {
	var missing []string
	if opts.Name.Value == "" {
		missing = append(missing, "--"+FlagName)
	}

	if opts.Mode == agentK8s.ModeWorker {
		if len(opts.WorkerPools.Value) == 0 {
			missing = append(missing, "--"+sharedWorker.FlagWorkerPool)
		}
	} else {
		if len(opts.Environments.Value) == 0 {
			missing = append(missing, "--"+sharedTarget.FlagEnvironment)
		}
		if len(opts.Roles.Value) == 0 && len(opts.Tags.Value) == 0 {
			missing = append(missing, fmt.Sprintf("--%s or --%s", sharedTarget.FlagRole, sharedTarget.FlagTag))
		}
	}

	// A dry run renders manifests without registering anything, so there is no
	// agreement being entered into.
	if !opts.AcceptEula.Value && !opts.DryRun.Value {
		missing = append(missing, "--"+FlagAcceptEula)
	}

	if len(missing) > 0 {
		return fmt.Errorf("%s must be specified when prompting is disabled", strings.Join(missing, ", "))
	}
	return nil
}

func (opts *InstallOptions) ResolveWithoutPrompting() error {
	if err := opts.resolvePollingAddresses(); err != nil {
		return err
	}

	if err := opts.resolveNames(); err != nil {
		return err
	}

	if err := opts.validateMachinePolicy(); err != nil {
		return err
	}

	if err := opts.validateWorkerPools(); err != nil {
		return err
	}

	if err := opts.resolveScriptPodRoles(context.Background()); err != nil {
		return err
	}

	opts.deriveAccessMode()
	opts.warnAboutAccessMode()
	return nil
}

// validateWorkerPools catches a pool that cannot hold a worker before the agent
// tries to register with it. A dynamic pool is the likely mistake: Octopus runs
// those on its own machines, so a worker cannot join one.
func (opts *InstallOptions) validateWorkerPools() error {
	if !opts.isWorker() || len(opts.WorkerPools.Value) == 0 {
		return nil
	}

	pools, err := opts.GetAllWorkerPoolsCallback()
	if err != nil {
		return err
	}

	known := map[string]bool{}
	for _, pool := range pools {
		known[strings.ToLower(pool.Name)] = true
		known[strings.ToLower(pool.ID)] = true
	}

	var unknown []string
	for _, given := range opts.WorkerPools.Value {
		if !known[strings.ToLower(strings.TrimSpace(given))] {
			unknown = append(unknown, given)
		}
	}
	if len(unknown) == 0 {
		return nil
	}

	return fmt.Errorf("no worker pool named %s in space %s can hold a worker. %s",
		strings.Join(unknown, ", "), opts.spaceName(), staticPoolAdvice())
}

// staticPoolAdvice is worth spelling out because a space can easily have only
// dynamic pools, which is the default on Octopus Cloud.
func staticPoolAdvice() string {
	return fmt.Sprintf("A Kubernetes worker joins a static worker pool; create one with `%s worker-pool static create`",
		constants.ExecutableName)
}

// validateMachinePolicy checks the name before anything is created, so a typo
// surfaces here rather than in a registration that fails inside the cluster.
func (opts *InstallOptions) validateMachinePolicy() error {
	if opts.MachinePolicy.Value == "" {
		return nil
	}
	_, err := machinescommon.FindMachinePolicy(opts.GetAllMachinePoliciesCallback, opts.MachinePolicy.Value)
	return err
}

// resolveScriptPodRoles copies the rules out of the named roles. They are
// copied rather than referenced because the chart takes rules, so this is a
// snapshot: the agent does not follow the roles afterwards.
func (opts *InstallOptions) resolveScriptPodRoles(ctx context.Context) error {
	if len(opts.ScriptPodRoles.Value) == 0 {
		return nil
	}
	if opts.RestrictScriptPods.Value {
		return fmt.Errorf("--%s and --%s ask for opposite things; script pods either have no permissions of their own or the ones copied from %s",
			FlagRestrictScriptPods, FlagScriptPodRole, strings.Join(opts.ScriptPodRoles.Value, ", "))
	}

	roles := make([]octoK8s.Role, 0, len(opts.ScriptPodRoles.Value))
	for _, reference := range opts.ScriptPodRoles.Value {
		role, err := opts.Cluster.FindRole(ctx, reference)
		if err != nil {
			return err
		}
		roles = append(roles, role)
	}

	opts.ScriptPodRules = octoK8s.MergePolicyRules(roles)
	if len(opts.ScriptPodRules) == 0 {
		return fmt.Errorf("%s grants nothing, so copying it would leave script pods unable to deploy. "+
			"Use --%s if that is what you want", strings.Join(opts.ScriptPodRoles.Value, ", "), FlagRestrictScriptPods)
	}
	return nil
}

// resolvedStorageClass is where the volume actually comes from: the class that
// was chosen, or the cluster's default when none was.
func (opts *InstallOptions) resolvedStorageClass() (octoK8s.StorageClass, bool) {
	for _, class := range opts.StorageClasses {
		if opts.StorageClass.Value == "" {
			if class.IsDefault {
				return class, true
			}
			continue
		}
		if class.Name == opts.StorageClass.Value {
			return class, true
		}
	}
	return octoK8s.StorageClass{}, false
}

// deriveAccessMode reads the access mode off the storage class rather than
// asking for it. Whether script pods can spread across nodes follows from
// whether the class serves a shared filesystem, which is not a separate
// decision anybody should have to make.
func (opts *InstallOptions) deriveAccessMode() {
	if opts.AccessModeChosen {
		return
	}
	class, found := opts.resolvedStorageClass()
	opts.ReadWriteMany.Value = found && class.SupportsReadWriteMany()
}

// warnAboutAccessMode is a warning rather than a refusal: the provisioner is
// only a signal, and a class this does not recognise may well serve a shared
// filesystem.
func (opts *InstallOptions) warnAboutAccessMode() {
	if !opts.ReadWriteMany.Value || !opts.AccessModeChosen {
		return
	}

	class, found := opts.resolvedStorageClass()
	if found && class.SupportsReadWriteMany() {
		return
	}

	fmt.Fprintf(opts.Out, "%s --%s asks for a ReadWriteMany volume from %s, which is not known to serve one. "+
		"If it cannot, the volume never binds and the agent stays pending.\n",
		output.Yellow("!"), FlagReadWriteMany, storageClassDescription(class, found, opts.StorageClass.Value))
}

func storageClassDescription(class octoK8s.StorageClass, found bool, requested string) string {
	switch {
	case found && class.Provisioner != "":
		return fmt.Sprintf("%s (%s)", class.Name, class.Provisioner)
	case requested != "":
		return requested
	default:
		return "the cluster's default storage class"
	}
}

// resolvePollingAddresses fills in the polling address when none was given.
// Octopus Cloud and a single-node server have one derivable address; a High
// Availability cluster does not - each node needs its own address, which only
// the person who set the cluster up knows.
func (opts *InstallOptions) resolvePollingAddresses() error {
	if len(opts.ServerCommsAddresses.Value) > 0 {
		return nil
	}

	if nodes := opts.haNodes(); len(nodes) > 0 {
		names := make([]string, 0, len(nodes))
		for _, node := range nodes {
			names = append(names, node.Name)
		}
		return fmt.Errorf("this Octopus Server is a High Availability cluster (nodes %s), and the agent polls every node on its own address; give --%s once per node",
			strings.Join(names, ", "), FlagServerCommsAddress)
	}

	if derived := octoK8s.DerivePollingURL(opts.Host); derived != "" {
		opts.ServerCommsAddresses.Value = []string{derived}
	}
	return nil
}

// haNodes is empty unless Octopus is a self-hosted High Availability cluster.
// Octopus Cloud serves every polling connection on one shared address, so its
// nodes are its own business.
func (opts *InstallOptions) haNodes() []octopusservernodes.Node {
	if octoK8s.IsOctopusCloud(opts.Host) {
		return nil
	}
	nodes := opts.taskNodes()
	if len(nodes) <= 1 {
		return nil
	}
	return nodes
}

// taskNodes degrades to none rather than failing: the topology read is a
// convenience, and a credential that cannot read it can still name the polling
// addresses itself.
func (opts *InstallOptions) taskNodes() []octopusservernodes.Node {
	if opts.serverNodesRead || opts.ServerNodesCallback == nil {
		return opts.serverNodes
	}
	opts.serverNodesRead = true

	nodes, err := opts.ServerNodesCallback()
	if err != nil {
		fmt.Fprintf(opts.Out, "%s Could not read the Octopus Server's nodes to check for High Availability: %v\n",
			output.Yellow("!"), err)
		return nil
	}
	opts.serverNodes = nodes
	return nodes
}

func (opts *InstallOptions) resolveNames() error {
	namespace, release, err := octoK8s.ResolveNames(opts.Namespace.Value, opts.ReleaseName.Value, opts.namespacePrefix(), opts.Name.Value)
	if err != nil {
		return err
	}
	opts.TargetNamespace, opts.TargetRelease = namespace, release
	return nil
}

// namespacePrefix matches what the Octopus portal generates for each mode, so
// a CLI install and a portal install of the same name land in the same place.
func (opts *InstallOptions) namespacePrefix() string {
	if opts.isWorker() {
		return octoK8s.WorkerNamespacePrefix
	}
	return octoK8s.AgentNamespacePrefix
}

func (opts *InstallOptions) spaceName() string {
	if name := opts.GetSpaceNameOrEmpty(); name != "" {
		return name
	}
	return "Default"
}

func (opts *InstallOptions) spaceID() string {
	if opts.Space == nil {
		return ""
	}
	return opts.Space.ID
}

func (opts *InstallOptions) isWorker() bool {
	return opts.Mode == agentK8s.ModeWorker
}

// installedThing names what goes into the cluster, which is one chart either
// way. Mode names what Octopus ends up with, which is what a name or a
// registration belongs to.
func (opts *InstallOptions) installedThing() string {
	if opts.isWorker() {
		return "Kubernetes worker"
	}
	return "Kubernetes agent"
}

// existingRelease is the agent this install would replace, which is worth
// saying: a Helm release name is derived from the agent name, so reusing a name
// upgrades an agent rather than adding one.
func (opts *InstallOptions) existingRelease() (agentK8s.Installation, bool) {
	for _, installation := range opts.Installations {
		if installation.Release.Name == opts.TargetRelease && installation.Release.Namespace == opts.TargetNamespace {
			return installation, true
		}
	}
	return agentK8s.Installation{}, false
}

// registered answers whether Octopus already has an agent of this name.
// Registration matches on name, so an existing one is taken over rather than
// added to.
func registered(dependencies *cmd.Dependencies, mode agentK8s.Mode, name string) (bool, error) {
	if dependencies.Client == nil || strings.TrimSpace(name) == "" {
		return false, nil
	}
	spaceID := ""
	if dependencies.Space != nil {
		spaceID = dependencies.Space.ID
	}

	if mode == agentK8s.ModeWorker {
		page, err := workers.Get(dependencies.Client, spaceID, machines.WorkersQuery{PartialName: name})
		if err != nil {
			return false, err
		}
		for _, worker := range page.Items {
			if strings.EqualFold(worker.Name, name) {
				return true, nil
			}
		}
		return false, nil
	}

	page, err := machines.Get(dependencies.Client, spaceID, machines.MachinesQuery{PartialName: name})
	if err != nil {
		return false, err
	}
	for _, target := range page.Items {
		if strings.EqualFold(target.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

// knownTargetTags is read once and kept, so the review can tell a tag the space
// already had from one that will be created.
func (opts *InstallOptions) knownTargetTags() ([]string, error) {
	if opts.KnownTargetTags != nil || opts.TargetTagsCallback == nil {
		return opts.KnownTargetTags, nil
	}

	tags, err := opts.TargetTagsCallback()
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	opts.KnownTargetTags = tags
	return tags, nil
}

// newTargetTags are the chosen tags Octopus has never seen, which it creates
// when the agent registers. Tags that came from a flag are taken at face value:
// nothing was read, so nothing can be called new.
func (opts *InstallOptions) newTargetTags() []string {
	if opts.KnownTargetTags == nil {
		return nil
	}

	known := map[string]bool{}
	for _, tag := range opts.KnownTargetTags {
		known[tag] = true
	}

	var created []string
	for _, tag := range opts.Roles.Value {
		if !known[tag] {
			created = append(created, tag)
		}
	}
	return created
}

func shortDescription(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return "Install the Octopus Kubernetes agent as a worker"
	}
	return "Install the Octopus Kubernetes agent as a deployment target"
}

func longDescription(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return heredoc.Doc(`
			Install the Octopus Kubernetes agent into a Kubernetes cluster as a worker.

			The worker runs Octopus steps in the cluster, one pod per task, and releases the
			compute again when the task finishes. It polls Octopus for work, so the cluster does
			not need to be reachable from outside.

			Run without arguments to be prompted. Anything that can be read from the cluster or
			from Octopus is filled in for you: the Octopus server, space and polling address, the
			cluster's storage classes, and the install namespace.
		`)
	}

	return heredoc.Doc(`
		Install the Octopus Kubernetes agent into a Kubernetes cluster as a deployment target.

		The agent runs Kubernetes steps from inside the cluster, so Octopus does not need
		cluster credentials and the cluster does not need to be reachable from outside - the
		agent polls Octopus for work.

		Run without arguments to be prompted. Anything that can be read from the cluster or
		from Octopus is filled in for you: the Octopus server, space and polling address, the
		cluster's storage classes, and the install namespace.
	`)
}

func examples(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return heredoc.Docf(`
			$ %[1]s kubernetes worker install
			$ %[1]s kubernetes worker install --name cluster-worker --worker-pool "Kubernetes Pool" --dry-run
			$ %[1]s kubernetes worker install --name cluster-worker --worker-pool "Kubernetes Pool" --accept-eula --no-prompt
		`, constants.ExecutableName)
	}

	return heredoc.Docf(`
		$ %[1]s kubernetes agent install
		$ %[1]s kubernetes agent install --name production --environment Production --role k8s --dry-run
		$ %[1]s kubernetes agent install --name production --environment Production --role k8s --accept-eula --no-prompt
	`, constants.ExecutableName)
}

func (opts *InstallOptions) chartRef() helm.ChartRef {
	return agentK8s.ChartRef.WithVersion(opts.ChartVersion.Value)
}

var errEulaDeclined = errors.New("the Octopus Customer Agreement has to be accepted to install the agent")

func (opts *InstallOptions) reportUnsupportedNodes() {
	unsupported := agentK8s.UnsupportedArchitectures(opts.NodeArchitectures)
	if len(unsupported) == 0 {
		return
	}
	fmt.Fprintf(opts.Out, "%s This cluster has %s nodes, which the agent cannot run on. It will only schedule on the linux/amd64 and linux/arm64 nodes.\n",
		output.Yellow("!"), strings.Join(unsupported, " and "))
}
