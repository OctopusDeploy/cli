package machinescommon

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/proxies"
	"github.com/spf13/cobra"
	"strings"
)

type GetAllProxiesCallback func() ([]*proxies.Proxy, error)

func PromptForProxy(opts *CreateTargetProxyOptions, flags *CreateTargetProxyFlags) error {
	if flags.Proxy.Value == "" {
		directConnection := true
		opts.Ask(&survey.Confirm{
			Message: "Should the connection to the tentacle be direct?",
			Default: true,
		}, &directConnection)
		if !directConnection {
			selectedOption, err := selectors.Select(opts.Ask, "Select the proxy to use", opts.GetAllProxiesCallback, func(p *proxies.Proxy) string { return p.GetName() })
			if err != nil {
				return err
			}
			flags.Proxy.Value = selectedOption.GetName()
		}
	}
	return nil
}

type CreateTargetProxyFlags struct {
	Proxy *flag.Flag[string]
}

type CreateTargetProxyOptions struct {
	*cmd.Dependencies
	GetAllProxiesCallback
}

func NewCreateTargetProxyFlags() *CreateTargetProxyFlags {
	return &CreateTargetProxyFlags{
		Proxy: flag.New[string](shared.FlagProxy, false),
	}
}

func NewCreateTargetProxyOptions(dependencies *cmd.Dependencies) *CreateTargetProxyOptions {
	return &CreateTargetProxyOptions{
		Dependencies: dependencies,
		GetAllProxiesCallback: func() ([]*proxies.Proxy, error) {
			return getAllProxies(*dependencies.Client)
		},
	}
}

func RegisterCreateTargetProxyFlags(cmd *cobra.Command, proxyFlags *CreateTargetProxyFlags) {
	cmd.Flags().StringVar(&proxyFlags.Proxy.Value, shared.FlagProxy, "", "Select whether to use a proxy to connect to this Tentacle. If omitted, will connect directly.")
}

func FindProxy(opts *CreateTargetProxyOptions, flags *CreateTargetProxyFlags) (*proxies.Proxy, error) {
	allProxy, err := opts.Client.Proxies.GetAll()
	if err != nil {
		return nil, err
	}
	var proxy *proxies.Proxy
	for _, p := range allProxy {
		if strings.EqualFold(p.GetID(), flags.Proxy.Value) || strings.EqualFold(p.GetName(), flags.Proxy.Value) {
			proxy = p
			break
		}
	}
	if proxy == nil {
		return nil, fmt.Errorf("cannot find proxy '%s'", flags.Proxy.Value)
	}
	return proxy, nil
}

func getAllProxies(client client.Client) ([]*proxies.Proxy, error) {
	res, err := client.Proxies.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
