package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/proxies"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProxyFlagSupplied_ShouldNotPrompt(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetProxyFlags()
	flags.Proxy.Value = "MachineProxy-1"

	opts := shared.NewCreateTargetProxyOptions(&cmd.Dependencies{Ask: asker})

	err := shared.PromptForProxy(opts, flags)
	checkRemainingPrompts()

	assert.NoError(t, err)
}

func TestNoProxyFlag_ShouldPrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Should the connection to the tentacle be direct?", "", false, true),
		testutil.NewSelectPrompt("Select the proxy to use", "", []string{"Proxy 1", "Proxy 2"}, "Proxy 2"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetProxyFlags()
	opts := shared.NewCreateTargetProxyOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllProxiesCallback = func() ([]*proxies.Proxy, error) {
		return []*proxies.Proxy{
			proxies.NewProxy("Proxy 1", "example.com", "user", core.NewSensitiveValue("password")),
			proxies.NewProxy("Proxy 2", "example2.com", "user", core.NewSensitiveValue("password")),
		}, nil
	}

	err := shared.PromptForProxy(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Proxy 2", flags.Proxy.Value)
}

func TestNoProxyFlag_DirectConnection(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Should the connection to the tentacle be direct?", "", true, true),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := shared.NewCreateTargetProxyFlags()
	opts := shared.NewCreateTargetProxyOptions(&cmd.Dependencies{Ask: asker})
	opts.GetAllProxiesCallback = func() ([]*proxies.Proxy, error) {
		return []*proxies.Proxy{
			proxies.NewProxy("Proxy 1", "example.com", "user", core.NewSensitiveValue("password")),
			proxies.NewProxy("Proxy 2", "example2.com", "user", core.NewSensitiveValue("password")),
		}, nil
	}

	err := shared.PromptForProxy(opts, flags)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Empty(t, flags.Proxy.Value)
}
