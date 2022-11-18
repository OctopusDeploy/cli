package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	listeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/worker/listening-tentacle/view"
	pollingTentacle "github.com/OctopusDeploy/cli/pkg/cmd/worker/polling-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	ssh "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View a worker in an instance of Octopus Deploy",
		Long:  "View a worker in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s worker view Machines-100
			$ %s worker view 'worker'
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var worker, err = opts.Client.Workers.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	switch worker.Endpoint.GetCommunicationStyle() {
	case "TentaclePassive":
		return listeningTentacle.ViewRun(opts)
	case "TentacleActive":
		return pollingTentacle.ViewRun(opts)
	case "Ssh":
		return ssh.ViewRun(opts)
	default:
		return fmt.Errorf("unsupported worker '%s'", worker.Endpoint.GetCommunicationStyle())
	}

	return nil
}
