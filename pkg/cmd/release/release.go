package release

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/release/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdRelease(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "release <command>",
		Short:   "Manage releases",
		Long:    `Work with Octopus Deploy releases.`,
		Example: fmt.Sprintf("$ %s release list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	return cmd
}
