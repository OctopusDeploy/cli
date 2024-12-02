package generic_oidc

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/generic-oidc/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/generic-oidc/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdGenericOidc(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generic-oidc <command>",
		Short:   "Manage Generic OpenID Connect accounts",
		Long:    "Manage Generic OpenID Connect accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account generic-oidc list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
