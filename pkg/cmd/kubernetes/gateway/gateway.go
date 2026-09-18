package gateway

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	cmdRotateToken "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/rotatetoken"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdGateway(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway <command>",
		Short: "Manage the Octopus Argo CD gateway",
		Long: heredoc.Doc(`
			Manage the Octopus Argo CD gateway, which connects an Argo CD instance to Octopus.
		`),
		Example: heredoc.Docf("$ %s kubernetes gateway install", constants.ExecutableName),
	}

	cmd.AddCommand(cmdInstall.NewCmdInstall(f))
	cmd.AddCommand(cmdRotateToken.NewCmdRotateToken(f))

	return cmd
}
