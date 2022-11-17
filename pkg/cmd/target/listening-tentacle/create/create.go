package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"net/url"
)

const (
	FlagName       = "name"
	FlagThumbprint = "thumbprint"
	FlagUrl        = "url"
)

type CreateFlags struct {
	Name       *flag.Flag[string]
	Thumbprint *flag.Flag[string]
	URL        *flag.Flag[string]
	*machinescommon.CreateTargetProxyFlags
	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*machinescommon.CreateTargetMachinePolicyFlags
	*shared.CreateTargetTenantFlags
	*machinescommon.WebFlags
}

type CreateOptions struct {
	*CreateFlags
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
		Thumbprint:                     flag.New[string](FlagThumbprint, true),
		URL:                            flag.New[string](FlagUrl, false),
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
		Use:   "create",
		Short: "Create a listening tentacle deployment target",
		Long:  "Create a listening tentacle deployment target in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target listening-tentacle create
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this Listening Tentacle.")
	flags.StringVar(&createFlags.Thumbprint.Value, createFlags.Thumbprint.Name, "", "The X509 certificate thumbprint that securely identifies the Tentacle.")
	flags.StringVar(&createFlags.URL.Value, createFlags.URL.Name, "", "The network address at which the Tentacle can be reached.")
	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
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

	url, err := url.Parse(opts.URL.Value)
	if err != nil {
		return err
	}

	envs, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}
	environmentIds := util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })

	endpoint := machines.NewListeningTentacleEndpoint(url, opts.Thumbprint.Value)
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

	fmt.Fprintf(opts.Out, "Successfully created listening tenatcle '%s'.\n", deploymentTarget.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.URL, opts.Thumbprint, opts.Environments, opts.Roles, opts.Proxy, opts.MachinePolicy, opts.TenantedDeploymentMode, opts.Tenants, opts.TenantTags)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForTargets(createdTarget, opts.Dependencies, opts.WebFlags, "listening tentacle")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Listening Tentacle", &opts.Name.Value)
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

	if opts.Thumbprint.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Thumbprint",
			Help:    "The X509 certificate thumbprint that securely identifies the Tentacle.",
		}, &opts.Thumbprint.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MinLength(40),
			survey.MaxLength(40),
		))); err != nil {
			return err
		}
	}

	if opts.URL.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "URL",
			Help:    "The network address at which the Tentacle can be reached.",
		}, &opts.URL.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	err = machinescommon.PromptForMachinePolicy(opts.CreateTargetMachinePolicyOptions, opts.CreateTargetMachinePolicyFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}
