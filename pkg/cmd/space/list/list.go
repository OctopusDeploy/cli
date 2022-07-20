package list

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewCmdList(f apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces in an instance of Octopus Deploy",
		Long:  "List spaces in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(f, cmd.OutOrStdout())
		},
	}

	return cmd
}

func listRun(f apiclient.ClientFactory, w io.Writer) error {
	client, err := f.Get(true)
	if err != nil {
		return err
	}

	allSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	t := output.NewTable(w)
	t.AddRow("NAME", "DESCRIPTION", "TASK QUEUE")

	for _, space := range allSpaces {
		name := output.Bold(space.Name)
		taskQueue := output.Green("Running")
		if space.TaskQueueStopped {
			taskQueue = output.Yellow("Stopped")
		}
		t.AddRow(name, space.Description, taskQueue)
	}

	return t.Print()
}
