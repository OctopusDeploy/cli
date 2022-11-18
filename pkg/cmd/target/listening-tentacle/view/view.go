package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
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
			return ViewRun(opts)
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeEndpoint, "Listening Tentacle")
}

func contributeEndpoint(opts *shared.ViewOptions, targetEndpoint machines.IEndpoint) ([]*shared.DataRow, error) {
	data := []*shared.DataRow{}

	endpoint := targetEndpoint.(*machines.ListeningTentacleEndpoint)
	data = append(data, shared.NewDataRow("URI", endpoint.URI.String()))
	data = append(data, shared.NewDataRow("Tentacle version", endpoint.TentacleVersionDetails.Version))

	proxyData, err := shared.ContributeProxy(opts, endpoint.ProxyID)
	if err != nil {
		return nil, err
	}
	data = append(data, proxyData...)

	return data, nil
}
