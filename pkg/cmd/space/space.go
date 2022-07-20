package space

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/space/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/space/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/space/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdSpace(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space <command>",
		Short: "Manage spaces",
		Long:  `Work with Octopus Deploy spaces.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list
			$ %s space view
		`), constants.ExecutableName, constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(client))
	cmd.AddCommand(cmdList.NewCmdList(client))
	cmd.AddCommand(cmdView.NewCmdView(client))

	return cmd
}
