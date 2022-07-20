package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewCmdList(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces in an instance of Octopus Deploy",
		Long:  "List spaces in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.Get(true)
			if err != nil {
				return err
			}

			allSpaces, err := client.Spaces.GetAll()
			if err != nil {
				return err
			}

			t := output.NewTable(cmd.OutOrStdout())
			t.AddRow("NAME", "DESCRIPTION", "TASK QUEUE")

			for _, space := range allSpaces {
				taskQueue := output.Green("Running")
				if space.TaskQueueStopped {
					taskQueue = output.Yellow("Stopped")
				}
				t.AddRow(space.Name, space.Description, taskQueue)
			}

			return t.Print()
		},
	}

	return cmd
}
