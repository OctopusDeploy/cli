package connect

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	connectTenant "github.com/OctopusDeploy/cli/pkg/cmd/tenant/connect"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdConnect(f factory.Factory) *cobra.Command {
	connectFlags := connectTenant.NewConnectFlags()
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a tenant to a project",
		Long:  "Connect a tenant to a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project connect
			$ %[1]s project connect --tenant "Bobs Wood Shop" --project "Deploy web site" --environment "Production"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := connectTenant.NewConnectOptions(connectFlags, cmd.NewDependencies(f, c))

			return connectTenant.ConnectRun(opts)
		},
	}

	connectTenant.ConfigureFlags(cmd, connectFlags)
	return cmd
}
