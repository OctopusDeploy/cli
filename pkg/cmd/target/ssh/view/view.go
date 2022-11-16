package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strings"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a SSH deployment target in an instance of Octopus Deploy",
		Long:  "View a SSH deployment target in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target ssh view 'linux-web-server'
			$ %s deployment-target ssh view Machines-100
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args)
			return viewRun(opts)
		},
	}

	shared.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func viewRun(opts *shared.ViewOptions) error {
	var target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}
	err = shared.ViewRun(opts, target)
	if err != nil {
		return err
	}

	endpoint := target.Endpoint.(*machines.SSHEndpoint)
	fmt.Fprintf(opts.Out, "URI: %s\n", endpoint.URI)
	err = shared.ViewAccount(opts, endpoint.AccountID)
	if err != nil {
		return err
	}

	err = shared.ViewProxy(opts, endpoint.ProxyID)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Runtime architecture: %s\n", getRuntimeArchitecture(endpoint))

	fmt.Fprintf(opts.Out, "\n")
	shared.DoWeb(target, opts.Dependencies, opts.WebFlags, "SSH")
	return nil
}

func getRuntimeArchitecture(endpoint *machines.SSHEndpoint) string {
	if endpoint.DotNetCorePlatform == "" {
		return "Mono"
	}

	return endpoint.DotNetCorePlatform
}

func getWebAppDisplay(endpoint *machines.AzureWebAppEndpoint) string {
	builder := &strings.Builder{}
	builder.WriteString(endpoint.WebAppName)
	if endpoint.WebAppSlotName != "" {
		builder.WriteString(fmt.Sprintf("/%s", endpoint.WebAppSlotName))
	}

	return builder.String()
}
