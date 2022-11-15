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
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

type GetAllAccountsForSshTarget func() ([]accounts.IAccount, error)

const (
	FlagName        = "name"
	FlagFingerprint = "fingerprint"
	FlagHost        = "host"
	FlagPort        = "port"
	FlagAccount     = "account"
	FlagRuntime     = "runtime"
	FlagPlatform    = "platform"

	SelfContainedCalamari = "self-contained"
	MonoCalamari          = "mono"

	LinuxX64   = "linux-x64"
	LinuxArm64 = "linux-arm64"
	LinuxArm   = "linux-arm"
	OsxX64     = "osx-x64"
)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Fingerprint *flag.Flag[string]
	HostName    *flag.Flag[string]
	Port        *flag.Flag[int]
	Account     *flag.Flag[string]
	Runtime     *flag.Flag[string]
	Platform    *flag.Flag[string]
	*shared.CreateTargetProxyFlags
	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*shared.CreateTargetMachinePolicyFlags
	*shared.CreateTargetTenantFlags
	*shared.WebFlags
}

type CreateOptions struct {
	*CreateFlags
	GetAllAccountsForSshTarget
	*shared.CreateTargetProxyOptions
	*shared.CreateTargetEnvironmentOptions
	*shared.CreateTargetRoleOptions
	*shared.CreateTargetMachinePolicyOptions
	*shared.CreateTargetTenantOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                           flag.New[string](FlagName, false),
		Account:                        flag.New[string](FlagAccount, false),
		Fingerprint:                    flag.New[string](FlagFingerprint, true),
		HostName:                       flag.New[string](FlagHost, false),
		Port:                           flag.New[int](FlagPort, false),
		Runtime:                        flag.New[string](FlagRuntime, false),
		Platform:                       flag.New[string](FlagPlatform, false),
		CreateTargetRoleFlags:          shared.NewCreateTargetRoleFlags(),
		CreateTargetProxyFlags:         shared.NewCreateTargetProxyFlags(),
		CreateTargetMachinePolicyFlags: shared.NewCreateTargetMachinePolicyFlags(),
		CreateTargetEnvironmentFlags:   shared.NewCreateTargetEnvironmentFlags(),
		CreateTargetTenantFlags:        shared.NewCreateTargetTenantFlags(),
		WebFlags:                       shared.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                      createFlags,
		Dependencies:                     dependencies,
		CreateTargetRoleOptions:          shared.NewCreateTargetRoleOptions(dependencies),
		CreateTargetProxyOptions:         shared.NewCreateTargetProxyOptions(dependencies),
		CreateTargetMachinePolicyOptions: shared.NewCreateTargetMachinePolicyOptions(dependencies),
		CreateTargetEnvironmentOptions:   shared.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetTenantOptions:        shared.NewCreateTargetTenantOptions(dependencies),

		GetAllAccountsForSshTarget: func() ([]accounts.IAccount, error) {
			return getAllAccountsForSshTarget(dependencies.Client)
		},
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a SSH deployment target",
		Long:  "Create a SSH deployment target in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target ssh create
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this Deployment Target.")
	flags.StringVar(&createFlags.Account.Value, createFlags.Account.Name, "", "The name or ID of the SSH key pair or username/password account")
	flags.StringVar(&createFlags.HostName.Value, createFlags.HostName.Name, "", "The hostname or IP address of the deployment target to connect to.")
	flags.StringVar(&createFlags.Fingerprint.Value, createFlags.Fingerprint.Name, "", "The host fingerprint of the deployment target.")
	flags.IntVar(&createFlags.Port.Value, createFlags.Port.Name, 0, "The port to connect to the deployment target on.")
	flags.StringVar(&createFlags.Runtime.Value, createFlags.Runtime.Name, "", fmt.Sprintf("The runtime to use to run Calamari on the deployment target. Options are '%s' or '%s'", SelfContainedCalamari, MonoCalamari))
	flags.StringVar(&createFlags.Platform.Value, createFlags.Platform.Name, "", fmt.Sprintf("The platform to use for the %s Calamari. Options are '%s', '%s', '%s' or '%s'", SelfContainedCalamari, LinuxX64, LinuxArm64, LinuxArm, OsxX64))

	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
	shared.RegisterCreateTargetRoleFlags(cmd, createFlags.CreateTargetRoleFlags)
	shared.RegisterCreateTargetProxyFlags(cmd, createFlags.CreateTargetProxyFlags)
	shared.RegisterCreateTargetMachinePolicyFlags(cmd, createFlags.CreateTargetMachinePolicyFlags)
	shared.RegisterCreateTargetTenantFlags(cmd, createFlags.CreateTargetTenantFlags)
	shared.RegisterWebFlag(cmd, createFlags.WebFlags)

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

	account, err := getAccount(opts)
	if err != nil {
		return err
	}

	port := opts.Port.Value
	if port == 0 {
		port = 22
	}
	endpoint := machines.NewSSHEndpoint(opts.HostName.Value, port, opts.Fingerprint.Value)
	endpoint.AccountID = account.GetID()

	if opts.Runtime.Value == SelfContainedCalamari {
		endpoint.DotNetCorePlatform = opts.Platform.Value
	} else {
		endpoint.DotNetCorePlatform = ""
	}

	if opts.Proxy.Value != "" {
		proxy, err := shared.FindProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
		if err != nil {
			return err
		}
		endpoint.ProxyID = proxy.GetID()
	}

	deploymentTarget := machines.NewDeploymentTarget(opts.Name.Value, endpoint, environmentIds, shared.DistinctRoles(opts.Roles.Value))
	machinePolicy, err := shared.FindMachinePolicy(opts.GetAllMachinePoliciesCallback, opts.MachinePolicy.Value)
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

	shared.DoWeb(createdTarget, opts.Dependencies, opts.WebFlags, "ssh")

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

	err = shared.PromptForMachinePolicy(opts.CreateTargetMachinePolicyOptions, opts.CreateTargetMachinePolicyFlags)
	if err != nil {
		return err
	}

	err = PromptForAccount(opts)
	if err != nil {
		return err
	}

	err = PromptForEndpoint(opts)
	if err != nil {
		return err
	}

	err = shared.PromptForProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
	if err != nil {
		return err
	}

	err = PromptForDotNetConfig(opts)
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}

func PromptForEndpoint(opts *CreateOptions) error {
	if opts.HostName.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Host",
			Help:    "The hostname or IP address at which the deployment target can be reached.",
		}, &opts.HostName.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if opts.Port.Value == 0 {
		var port string
		if err := opts.Ask(&survey.Input{
			Message: "Port",
			Help:    "Port number to connect over SSH to the deployment target. Default is 22",
		}, &port); err != nil {
			return err
		}

		if port == "" {
			port = "22"
		}

		if p, err := strconv.Atoi(port); err == nil {
			opts.Port.Value = p
		} else {
			return err
		}
	}

	if opts.Fingerprint.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Host fingerprint",
			Help:    "The host fingerprint of the SSH deployment target.",
		}, &opts.Fingerprint.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	return nil
}

func PromptForDotNetConfig(opts *CreateOptions) error {
	if opts.Runtime.Value == "" {
		selectedRuntime, err := selectors.SelectOptions(opts.Ask, "Select the target runtime\n", getTargetRuntimeOptions)
		if err != nil {
			return err
		}
		opts.Runtime.Value = selectedRuntime.Value
	}

	if opts.Runtime.Value == SelfContainedCalamari {
		if opts.Platform.Value == "" {
			selectedPlatform, err := selectors.SelectOptions(opts.Ask, "Select the target platform\n", getTargetPlatformOptions)
			if err != nil {
				return err
			}
			opts.Platform.Value = selectedPlatform.Value
		}
	}

	return nil
}

func PromptForAccount(opts *CreateOptions) error {
	var account accounts.IAccount
	if opts.Account.Value == "" {
		selectedAccount, err := selectors.Select(
			opts.Ask,
			"Select the account to use\n",
			opts.GetAllAccountsForSshTarget,
			func(a accounts.IAccount) string {
				return a.GetName()
			})
		if err != nil {
			return err
		}
		account = selectedAccount
	} else {
		a, err := getAccount(opts)
		if err != nil {
			return err
		}
		account = a
	}

	opts.Account.Value = account.GetName()
	return nil
}

func getAllAccountsForSshTarget(client *client.Client) ([]accounts.IAccount, error) {
	allAccounts, err := client.Accounts.GetAll()
	if err != nil {
		return nil, err
	}

	var accounts []accounts.IAccount
	for _, a := range allAccounts {
		if canBeUsedForSSH(a) {
			accounts = append(accounts, a)
		}
	}

	return accounts, nil
}

func canBeUsedForSSH(account accounts.IAccount) bool {
	accountType := account.GetAccountType()
	return accountType == accounts.AccountTypeSSHKeyPair || accountType == accounts.AccountTypeUsernamePassword
}

func getAccount(opts *CreateOptions) (accounts.IAccount, error) {
	idOrName := opts.Account.Value
	allAccounts, err := opts.GetAllAccountsForSshTarget()
	if err != nil {
		return nil, err
	}

	for _, a := range allAccounts {
		if strings.EqualFold(a.GetID(), idOrName) || strings.EqualFold(a.GetName(), idOrName) {
			return a, nil
		}
	}

	return nil, fmt.Errorf("cannot find account %s", idOrName)
}

func getTargetRuntimeOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Self-contained Calamari", Value: SelfContainedCalamari},
		{Display: "Calamari on Mono", Value: MonoCalamari},
	}
}

func getTargetPlatformOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Linux x64", Value: LinuxX64},
		{Display: "Linux ARM64", Value: LinuxArm64},
		{Display: "Linux ARM", Value: LinuxArm},
		{Display: "OSX x64", Value: OsxX64},
	}
}
