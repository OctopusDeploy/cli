package permissionscontroller

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/permissionscontroller/install"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPermissionsController(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permissions-controller <command>",
		Aliases: []string{"opc"},
		Short:   "Manage the Octopus permissions controller",
		Long: heredoc.Doc(`
			Manage the Octopus permissions controller, which decides which service account a
			Kubernetes agent's script pods run as.
		`),
		Example: heredoc.Docf("$ %s kubernetes permissions-controller install", constants.ExecutableName),
	}

	cmd.AddCommand(cmdInstall.NewCmdInstall(f))

	return cmd
}
