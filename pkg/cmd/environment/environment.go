package environment

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/environment/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdEnvironment(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "environment <command>",
		Aliases: []string{"env"},
		Short:   "Manage environments",
		Long:    `Work with Octopus Deploy environments.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment list
			$ %s env ls
		`), constants.ExecutableName, constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(client))
	return cmd
}
