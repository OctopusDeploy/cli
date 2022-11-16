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
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a Listening Tentacle deployment target in an instance of Octopus Deploy",
		Long:  "View a Listening Tentacle deployment target in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target listening-tentacle view 'EU'
			$ %s deployment-target listening-tentacle view Machines-100
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

	endpoint := target.Endpoint.(*machines.ListeningTentacleEndpoint)
	fmt.Fprintf(opts.Out, "URI: %s\n", endpoint.URI)
	fmt.Fprintf(opts.Out, "Tentacle version: %s\n", endpoint.TentacleVersionDetails.Version)
	if endpoint.ProxyID != "" {
		proxy, err := opts.Client.Proxies.GetById(endpoint.ProxyID)
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.Out, "Proxy: %s\n", proxy.GetName())
	} else {
		fmt.Println("No proxy configured")
	}

	fmt.Println()
	shared.DoWeb(target, opts.Dependencies, opts.WebFlags, "Listening Tentacle")
	return nil
}
