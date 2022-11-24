package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

type GetAllAccountsForSshTarget func() ([]accounts.IAccount, error)

const (
	FlagName = "name"
)

type CreateFlags struct {
	Name *flag.Flag[string]

	*machinescommon.CreateTargetProxyFlags
	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*machinescommon.CreateTargetMachinePolicyFlags
	*shared.CreateTargetTenantFlags
	*machinescommon.WebFlags
	*machinescommon.SshCommonFlags
}

type CreateOptions struct {
	*CreateFlags
	GetAllAccountsForSshTarget
	*machinescommon.SshCommonOptions
	*machinescommon.CreateTargetProxyOptions
	*shared.CreateTargetEnvironmentOptions
	*shared.CreateTargetRoleOptions
	*machinescommon.CreateTargetMachinePolicyOptions
	*shared.CreateTargetTenantOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                           flag.New[string](FlagName, false),
		SshCommonFlags:                 machinescommon.NewSshCommonFlags(),
		CreateTargetRoleFlags:          shared.NewCreateTargetRoleFlags(),
		CreateTargetProxyFlags:         machinescommon.NewCreateTargetProxyFlags(),
		CreateTargetMachinePolicyFlags: machinescommon.NewCreateTargetMachinePolicyFlags(),
		CreateTargetEnvironmentFlags:   shared.NewCreateTargetEnvironmentFlags(),
		CreateTargetTenantFlags:        shared.NewCreateTargetTenantFlags(),
		WebFlags:                       machinescommon.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                      createFlags,
		Dependencies:                     dependencies,
		SshCommonOptions:                 machinescommon.NewSshCommonOpts(dependencies),
		CreateTargetRoleOptions:          shared.NewCreateTargetRoleOptions(dependencies),
		CreateTargetProxyOptions:         machinescommon.NewCreateTargetProxyOptions(dependencies),
		CreateTargetMachinePolicyOptions: machinescommon.NewCreateTargetMachinePolicyOptions(dependencies),
		CreateTargetEnvironmentOptions:   shared.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetTenantOptions:        shared.NewCreateTargetTenantOptions(dependencies),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a SSH deployment target",
		Long:    "Create a SSH deployment target in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target ssh create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this deployment target.")

	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
	machinescommon.RegisterSshCommonFlags(cmd, createFlags.SshCommonFlags, "deployment target")
	shared.RegisterCreateTargetRoleFlags(cmd, createFlags.CreateTargetRoleFlags)
	machinescommon.RegisterCreateTargetProxyFlags(cmd, createFlags.CreateTargetProxyFlags)
	machinescommon.RegisterCreateTargetMachinePolicyFlags(cmd, createFlags.CreateTargetMachinePolicyFlags)
	shared.RegisterCreateTargetTenantFlags(cmd, createFlags.CreateTargetTenantFlags)
	machinescommon.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	envs, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}
	environmentIds := util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })

	account, err := machinescommon.GetSshAccount(opts.SshCommonOptions, opts.SshCommonFlags)
	if err != nil {
		return err
	}

	port := opts.Port.Value
	if port == 0 {
		port = 22
	}
	endpoint := machines.NewSSHEndpoint(opts.HostName.Value, port, opts.Fingerprint.Value)
	endpoint.AccountID = account.GetID()

	if opts.Runtime.Value == machinescommon.SelfContainedCalamari {
		endpoint.DotNetCorePlatform = opts.Platform.Value
	}

	if opts.Proxy.Value != "" {
		proxy, err := machinescommon.FindProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
		if err != nil {
			return err
		}
		endpoint.ProxyID = proxy.GetID()
	}

	deploymentTarget := machines.NewDeploymentTarget(opts.Name.Value, endpoint, environmentIds, shared.DistinctRoles(opts.Roles.Value))
	machinePolicy, err := machinescommon.FindMachinePolicy(opts.GetAllMachinePoliciesCallback, opts.MachinePolicy.Value)
	if err != nil {
		return err
	}
	deploymentTarget.MachinePolicyID = machinePolicy.GetID()

	err = shared.ConfigureTenant(deploymentTarget, opts.CreateTargetTenantFlags, opts.CreateTargetTenantOptions)
	if err != nil {
		return err
	}

	createdTarget, err := opts.Client.Machines.Add(deploymentTarget)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created SSH deployment target '%s'.\n", deploymentTarget.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.HostName, opts.Port, opts.Fingerprint, opts.Runtime, opts.Platform, opts.Environments, opts.Roles, opts.Account, opts.Proxy, opts.MachinePolicy, opts.TenantedDeploymentMode, opts.Tenants, opts.TenantTags)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForTargets(createdTarget, opts.Dependencies, opts.WebFlags, "ssh")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "SSH", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = shared.PromptForEnvironments(opts.CreateTargetEnvironmentOptions, opts.CreateTargetEnvironmentFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForRoles(opts.CreateTargetRoleOptions, opts.CreateTargetRoleFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForMachinePolicy(opts.CreateTargetMachinePolicyOptions, opts.CreateTargetMachinePolicyFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForSshAccount(opts.SshCommonOptions, opts.SshCommonFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForSshEndpoint(opts.SshCommonOptions, opts.SshCommonFlags, "deployment target")
	if err != nil {
		return err
	}

	err = machinescommon.PromptForProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForDotNetConfig(opts.SshCommonOptions, opts.SshCommonFlags, "deployment target")
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}
