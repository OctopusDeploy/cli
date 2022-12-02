package machinescommon_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForAccount_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()
	flags.Account.Value = "SSH keypair account"

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})
	opts.GetAllAccountsForSshMachine = func() ([]accounts.IAccount, error) {
		ssh, _ := accounts.NewSSHKeyAccount("SSH keypair account", "username", core.NewSensitiveValue("cert"))
		username, _ := accounts.NewUsernamePasswordAccount("Username account")
		return []accounts.IAccount{ssh, username}, nil
	}

	err := machinescommon.PromptForSshAccount(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForAccount_NoFlags(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the account to use\n", "", []string{"SSH keypair account", "Username account"}, "Username account"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})
	opts.GetAllAccountsForSshMachine = func() ([]accounts.IAccount, error) {
		ssh, _ := accounts.NewSSHKeyAccount("SSH keypair account", "username", core.NewSensitiveValue("cert"))
		username, _ := accounts.NewUsernamePasswordAccount("Username account")
		return []accounts.IAccount{ssh, username}, nil
	}

	err := machinescommon.PromptForSshAccount(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Username account", flags.Account.Value)
}

func TestPromptForDotNetConfig_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()
	flags.Runtime.Value = machinescommon.SelfContainedCalamari
	flags.Platform.Value = machinescommon.LinuxX64

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForDotNetConfig(opts, flags, "entity")
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForDotNetConfig_NoFlags_SelfContainedCalamari(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the target runtime\n", "", []string{"Self-contained Calamari", "Calamari on Mono"}, "Self-contained Calamari"),
		testutil.NewSelectPrompt("Select the target platform\n", "", []string{"Linux x64", "Linux ARM64", "Linux ARM", "OSX x64"}, "Linux x64"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForDotNetConfig(opts, flags, "target")
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, machinescommon.SelfContainedCalamari, flags.Runtime.Value)
	assert.Equal(t, machinescommon.LinuxX64, flags.Platform.Value)
}

func TestPromptForDotNetConfig_NoFlags_MonoCalamari(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the target runtime\n", "", []string{"Self-contained Calamari", "Calamari on Mono"}, "Calamari on Mono"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForDotNetConfig(opts, flags, "target")
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, machinescommon.MonoCalamari, flags.Runtime.Value)
	assert.Empty(t, flags.Platform.Value)
}

func TestPromptForEndpoint_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()
	flags.HostName.Value = "localhost"
	flags.Port.Value = 12000
	flags.Fingerprint.Value = "fingerprint"

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForSshEndpoint(opts, flags, "target")
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", flags.HostName.Value)
	assert.Equal(t, 12000, flags.Port.Value)
	assert.Equal(t, "fingerprint", flags.Fingerprint.Value)
}

func TestPromptForEndpoint_NoFlagSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Host", "The hostname or IP address at which the deployment target can be reached.", "localhost"),
		testutil.NewInputPromptWithDefault("Port", "Port number to connect over SSH to the deployment target. Default is 22", "22", ""),
		testutil.NewInputPrompt("Host fingerprint", "The host fingerprint of the SSH deployment target.", "fingerprint"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForSshEndpoint(opts, flags, "deployment target")
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", flags.HostName.Value)
	assert.Equal(t, 22, flags.Port.Value)
	assert.Equal(t, "fingerprint", flags.Fingerprint.Value)
}

func TestPromptForEndpoint_NoPortSupplied_ShouldSelectDefault(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPromptWithDefault("Port", "Port number to connect over SSH to the deployment target. Default is 22", "22", ""),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := machinescommon.NewSshCommonFlags()
	flags.HostName.Value = "localhost"
	flags.Fingerprint.Value = "fingerprint"

	opts := machinescommon.NewSshCommonOpts(&cmd.Dependencies{Ask: asker})

	err := machinescommon.PromptForSshEndpoint(opts, flags, "deployment target")
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", flags.HostName.Value)
	assert.Equal(t, 22, flags.Port.Value)
	assert.Equal(t, "fingerprint", flags.Fingerprint.Value)
}
