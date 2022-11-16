package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	azureWebApp "github.com/OctopusDeploy/cli/pkg/cmd/target/azure-web-app/view"
	cloudRegion "github.com/OctopusDeploy/cli/pkg/cmd/target/cloud-region/view"
	listeningTentacle "github.com/OctopusDeploy/cli/pkg/cmd/target/listening-tentacle/view"
	pollingTentacle "github.com/OctopusDeploy/cli/pkg/cmd/target/polling-tentacle/view"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	ssh "github.com/OctopusDeploy/cli/pkg/cmd/target/ssh/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View a deployment target in an instance of Octopus Deploy",
		Long:  "View a deployment target in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target view Machines-100
			$ %s deployment-target view 'web-server'
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args))
		},
	}

	shared.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	switch target.Endpoint.GetCommunicationStyle() {
	case "None":
		return cloudRegion.ViewRun(opts)
	case "TentaclePassive":
		return listeningTentacle.ViewRun(opts)
	case "TentacleActive":
		return pollingTentacle.ViewRun(opts)
	case "Ssh":
		return ssh.ViewRun(opts)
	case "OfflineDrop":
		return shared.ViewRun(opts, target)
	case "AzureWebApp":
		return azureWebApp.ViewRun(opts)
	case "AzureCloudService":
		return shared.ViewRun(opts, target)
	case "AzureServiceFabricCluster":
		return shared.ViewRun(opts, target)
	case "Kubernetes":
		return shared.ViewRun(opts, target)
	case "StepPackage":
		return shared.ViewRun(opts, target)
	default:
		return fmt.Errorf("unsupported deployment target '%s'", target.Endpoint.GetCommunicationStyle())
	}

	return nil
}
