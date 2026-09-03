package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/octopusservernodes"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workers"
)

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

func (opts *InstallOptions) ResolveWithoutPrompting(ctx context.Context) error {
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

	if err := opts.resolveScriptPodRoles(ctx); err != nil {
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
