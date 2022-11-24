package machinescommon

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

type GetAllAccountsForSshMachine func() ([]accounts.IAccount, error)

const (
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

	DefaultPort = 22
)

type SshCommonFlags struct {
	Fingerprint *flag.Flag[string]
	HostName    *flag.Flag[string]
	Port        *flag.Flag[int]
	Account     *flag.Flag[string]
	Runtime     *flag.Flag[string]
	Platform    *flag.Flag[string]
}

type SshCommonOptions struct {
	*cmd.Dependencies
	GetAllAccountsForSshMachine
}

func NewSshCommonFlags() *SshCommonFlags {
	return &SshCommonFlags{
		Account:     flag.New[string](FlagAccount, false),
		Fingerprint: flag.New[string](FlagFingerprint, true),
		HostName:    flag.New[string](FlagHost, false),
		Port:        flag.New[int](FlagPort, false),
		Runtime:     flag.New[string](FlagRuntime, false),
		Platform:    flag.New[string](FlagPlatform, false),
	}
}

func NewSshCommonOpts(dependencies *cmd.Dependencies) *SshCommonOptions {
	return &SshCommonOptions{
		Dependencies: dependencies,
		GetAllAccountsForSshMachine: func() ([]accounts.IAccount, error) {
			return getAllAccountsForSshMachine(dependencies.Client)
		},
	}
}

func RegisterSshCommonFlags(cmd *cobra.Command, flags *SshCommonFlags, entityType string) {
	cmd.Flags().StringVar(&flags.Account.Value, flags.Account.Name, "", "The name or ID of the SSH key pair or username/password account")
	cmd.Flags().StringVar(&flags.HostName.Value, flags.HostName.Name, "", fmt.Sprintf("The hostname or IP address of the %s to connect to.", entityType))
	cmd.Flags().StringVar(&flags.Fingerprint.Value, flags.Fingerprint.Name, "", fmt.Sprintf("The host fingerprint of the %s.", entityType))
	cmd.Flags().IntVar(&flags.Port.Value, flags.Port.Name, 0, fmt.Sprintf("The port to connect to the %s on.", entityType))
	cmd.Flags().StringVar(&flags.Runtime.Value, flags.Runtime.Name, "", fmt.Sprintf("The runtime to use to run Calamari on the %s. Options are '%s' or '%s'", entityType, SelfContainedCalamari, MonoCalamari))
	cmd.Flags().StringVar(&flags.Platform.Value, flags.Platform.Name, "", fmt.Sprintf("The platform to use for the %s Calamari. Options are '%s', '%s', '%s' or '%s'", SelfContainedCalamari, LinuxX64, LinuxArm64, LinuxArm, OsxX64))

}

func PromptForSshEndpoint(opts *SshCommonOptions, flags *SshCommonFlags, entityType string) error {
	if flags.HostName.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Host",
			Help:    fmt.Sprintf("The hostname or IP address at which the %s  can be reached.", entityType),
		}, &flags.HostName.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if flags.Port.Value == 0 {
		var port string
		if err := opts.Ask(&survey.Input{
			Message: "Port",
			Help:    fmt.Sprintf("Port number to connect over SSH to the %s. Default is %d", entityType, DefaultPort),
			Default: fmt.Sprintf("%d", DefaultPort),
		}, &port); err != nil {
			return err
		}

		if port == "" {
			port = fmt.Sprintf("%d", DefaultPort)
		}

		if p, err := strconv.Atoi(port); err == nil {
			flags.Port.Value = p
		} else {
			return err
		}
	}

	if flags.Fingerprint.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Host fingerprint",
			Help:    fmt.Sprintf("The host fingerprint of the SSH %s.", entityType),
		}, &flags.Fingerprint.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	return nil
}

func PromptForDotNetConfig(opts *SshCommonOptions, flags *SshCommonFlags, entityType string) error {
	if flags.Runtime.Value == "" {
		selectedRuntime, err := selectors.SelectOptions(opts.Ask, fmt.Sprintf("Select the %s runtime\n", entityType), getTargetRuntimeOptions)
		if err != nil {
			return err
		}
		flags.Runtime.Value = selectedRuntime.Value
	}

	if flags.Runtime.Value == SelfContainedCalamari {
		if flags.Platform.Value == "" {
			selectedPlatform, err := selectors.SelectOptions(opts.Ask, fmt.Sprintf("Select the %s platform\n", entityType), getTargetPlatformOptions)
			if err != nil {
				return err
			}
			flags.Platform.Value = selectedPlatform.Value
		}
	}

	return nil
}

func PromptForSshAccount(opts *SshCommonOptions, flags *SshCommonFlags) error {
	var account accounts.IAccount
	if flags.Account.Value == "" {
		selectedAccount, err := selectors.Select(
			opts.Ask,
			"Select the account to use\n",
			opts.GetAllAccountsForSshMachine,
			func(a accounts.IAccount) string {
				return a.GetName()
			})
		if err != nil {
			return err
		}
		account = selectedAccount
	} else {
		a, err := GetSshAccount(opts, flags)
		if err != nil {
			return err
		}
		account = a
	}

	flags.Account.Value = account.GetName()
	return nil
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

func getAllAccountsForSshMachine(client *client.Client) ([]accounts.IAccount, error) {
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

func GetSshAccount(opts *SshCommonOptions, flags *SshCommonFlags) (accounts.IAccount, error) {
	idOrName := flags.Account.Value
	allAccounts, err := opts.GetAllAccountsForSshMachine()
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
