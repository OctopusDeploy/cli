package space

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/space/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/space/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/space/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/space/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSpace(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space <command>",
		Short: "Manage spaces",
		Long:  `Work with Octopus Deploy spaces.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list
			$ %s space view
		`), constants.ExecutableName, constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))

	return cmd
}
