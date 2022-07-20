package space

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/space/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdSpace(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "space <command>",
		Aliases: []string{"env"},
		Short:   "Manage spaces",
		Long:    `Work with Octopus Deploy spaces.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list
			$ %s env ls
		`), constants.ExecutableName, constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(client))
	return cmd
}
