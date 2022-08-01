package aws

import (
	"fmt"

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
		Long:    `Work with Octopus Deploy aws accounts.`,
		Example: fmt.Sprintf("$ %s account aws list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	return cmd
}
