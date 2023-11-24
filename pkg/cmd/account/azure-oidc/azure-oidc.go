package azure

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/azure-oidc/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/azure-oidc/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAzureOidc(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "azure-oidc <command>",
		Short:   "Manage Azure OpenID Connect accounts",
		Long:    "Manage Azure OpenID Connect accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account azure-oidc list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
