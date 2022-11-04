package shared

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/proxies"
	"github.com/spf13/cobra"
)

type GetAllProxiesCallback func() ([]*proxies.Proxy, error)

func PromptForProxy(opts *CreateTargetProxyOptions, flags *CreateTargetProxyFlags) error {
	if flags.Proxy.Value == "" {
		var directConnection bool
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
		Proxy: flag.New[string](FlagProxy, false),
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
	cmd.Flags().StringVar(&proxyFlags.Proxy.Value, FlagProxy, "", "Select whether to use a proxy to connect to this Tentacle. If omitted, will connect directly.")
}

func getAllProxies(client client.Client) ([]*proxies.Proxy, error) {
	res, err := client.Proxies.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
