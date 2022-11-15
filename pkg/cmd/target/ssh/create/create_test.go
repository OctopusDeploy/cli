package create_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForAccount_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Account.Value = "SSH keypair account"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAccountsForSshTarget = func() ([]accounts.IAccount, error) {
		ssh, _ := accounts.NewSSHKeyAccount("SSH keypair account", "username", core.NewSensitiveValue("cert"))
		username, _ := accounts.NewUsernamePasswordAccount("Username account")
		return []accounts.IAccount{ssh, username}, nil
	}

	err := create.PromptForAccount(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForAccount_NoFlags(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the account to use\n", "", []string{"SSH keypair account", "Username account"}, "Username account"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllAccountsForSshTarget = func() ([]accounts.IAccount, error) {
		ssh, _ := accounts.NewSSHKeyAccount("SSH keypair account", "username", core.NewSensitiveValue("cert"))
		username, _ := accounts.NewUsernamePasswordAccount("Username account")
		return []accounts.IAccount{ssh, username}, nil
	}

	err := create.PromptForAccount(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Username account", flags.Account.Value)
}

func TestPromptForDotNetConfig_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Runtime.Value = create.SelfContainedCalamari
	flags.Platform.Value = create.LinuxX64

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForDotNetConfig(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
}

func TestPromptForDotNetConfig_NoFlags_SelfContainedCalamari(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the target runtime\n", "", []string{"Self-contained Calamari", "Calamari on Mono"}, "Self-contained Calamari"),
		testutil.NewSelectPrompt("Select the target platform\n", "", []string{"Linux x64", "Linux ARM64", "Linux ARM", "OSX x64"}, "Linux x64"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForDotNetConfig(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, create.SelfContainedCalamari, opts.Runtime.Value)
	assert.Equal(t, create.LinuxX64, opts.Platform.Value)
}

func TestPromptForDotNetConfig_NoFlags_MonoCalamari(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the target runtime\n", "", []string{"Self-contained Calamari", "Calamari on Mono"}, "Calamari on Mono"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForDotNetConfig(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, create.MonoCalamari, opts.Runtime.Value)
	assert.Empty(t, opts.Platform.Value)
}

func TestPromptForEndpoint_FlagsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.HostName.Value = "localhost"
	flags.Port.Value = 12000
	flags.Fingerprint.Value = "fingerprint"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForEndpoint(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", opts.HostName.Value)
	assert.Equal(t, 12000, opts.Port.Value)
	assert.Equal(t, "fingerprint", opts.Fingerprint.Value)
}

func TestPromptForEndpoint_NoFlagSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Host", "The hostname or IP address at which the deployment target can be reached.", "localhost"),
		testutil.NewInputPrompt("Port", "Port number to connect over SSH to the deployment target. Default is 22", ""),
		testutil.NewInputPrompt("Host fingerprint", "The host fingerprint of the SSH deployment target.", "fingerprint"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForEndpoint(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", opts.HostName.Value)
	assert.Equal(t, 22, opts.Port.Value)
	assert.Equal(t, "fingerprint", opts.Fingerprint.Value)
}

func TestPromptForEndpoint_NoPortSupplied_ShouldSelectDefault(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Port", "Port number to connect over SSH to the deployment target. Default is 22", ""),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.HostName.Value = "localhost"
	flags.Fingerprint.Value = "fingerprint"

	opts := create.NewCreateOptions(flags, &cmd.Dependencies{Ask: asker})

	err := create.PromptForEndpoint(opts)
	checkRemainingPrompts()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", opts.HostName.Value)
	assert.Equal(t, 22, opts.Port.Value)
	assert.Equal(t, "fingerprint", opts.Fingerprint.Value)
}
