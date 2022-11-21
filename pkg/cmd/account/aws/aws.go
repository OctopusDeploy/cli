package aws

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/aws/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/aws/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAws(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "aws <command>",
		Short:   "Manage AWS accounts",
		Long:    "Manage AWS accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account aws list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	return cmd
}
