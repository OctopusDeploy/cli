package disconnect

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	tenantDisconnect "github.com/OctopusDeploy/cli/pkg/cmd/tenant/disconnect"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdDisconnect(f factory.Factory) *cobra.Command {
	disconnectFlags := tenantDisconnect.NewDisconnectFlags()

	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect a tenant from a project in Octopus Deploy",
		Long:  "Disconnect a tenant from a project in Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s project disconnect
			$ %s project disconnect --tenant "Test Tenant" --project "Deploy web site" --confirm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := tenantDisconnect.NewDisconnectOptions(disconnectFlags, cmd.NewDependencies(f, c))
			return tenantDisconnect.DisconnectRun(opts)
		},
	}

	tenantDisconnect.ConfigureFlags(cmd, disconnectFlags)
	return cmd
}
