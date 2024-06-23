package view

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a SSH worker",
		Long:  "View a SSH worker in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker ssh view 'linux-worker'
			$ %[1]s worker ssh view Machines-100
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args)
			return ViewRun(opts)
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeEndpoint, "SSH")
}

func contributeEndpoint(opts *shared.ViewOptions, workerEndpoint machines.IEndpoint) ([]*output.DataRow, error) {
	data := []*output.DataRow{}
	endpoint := workerEndpoint.(*machines.SSHEndpoint)

	data = append(data, output.NewDataRow("URI", endpoint.URI.String()))
	data = append(data, output.NewDataRow("Runtime architecture", GetRuntimeArchitecture(endpoint)))
	accountRows, err := shared.ContributeAccount(opts, endpoint.AccountID)
	if err != nil {
		return nil, err
	}
	data = append(data, accountRows...)

	proxy, err := shared.ContributeProxy(opts, endpoint.ProxyID)
	data = append(data, proxy...)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func GetRuntimeArchitecture(endpoint *machines.SSHEndpoint) string {
	if endpoint.DotNetCorePlatform == "" {
		return "Mono"
	}

	return endpoint.DotNetCorePlatform
}
