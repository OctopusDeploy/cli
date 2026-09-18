package install

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/factory"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
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
