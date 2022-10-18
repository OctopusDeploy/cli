package tenant

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	cmdConnect "github.com/OctopusDeploy/cli/pkg/cmd/tenant/connect"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTenaant(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant <command>",
		Short: "Manage tenants",
		Long:  `Work with Octopus Deploy tenants.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant list
			$ %s tenant ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdConnect.NewCmdConnect(f))

	return cmd
}
