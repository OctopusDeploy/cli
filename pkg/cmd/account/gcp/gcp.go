package gcp

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdGcp(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gcp <command>",
		Short:   "Manage Google Cloud accounts",
		Long:    "Manage Google Cloud accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account gcp list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
