package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces in an instance of Octopus Deploy",
		Long:  "List spaces in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(f, cmd)
		},
	}

	return cmd
}

func listRun(f factory.Factory, cmd *cobra.Command) error {
	client, err := f.Client(false)
	if err != nil {
		return err
	}

	allSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	type SpaceAsJson struct {
		Id          string `json:"Id"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
		TaskQueue   string `json:"TaskQueue"`
	}

	return output.PrintArray(allSpaces, cmd, output.Mappers[*spaces.Space]{
		Json: func(item *spaces.Space) any {
			taskQueue := "Running"
			if item.TaskQueueStopped {
				taskQueue = "Stopped"
			}
			return SpaceAsJson{
				Id:          item.GetID(),
				Name:        item.Name,
				Description: item.Description,
				TaskQueue:   taskQueue,
			}
		},
		Table: output.TableDefinition[*spaces.Space]{
			Header: []string{"NAME", "DESCRIPTION", "TASK QUEUE"},
			Row: func(item *spaces.Space) []string {
				taskQueue := output.Green("Running")
				if item.TaskQueueStopped {
					taskQueue = output.Yellow("Stopped")
				}

				return []string{output.Bold(item.Name), item.Description, taskQueue}
			},
		},
		Basic: func(item *spaces.Space) string {
			return item.Name
		},
	})
}
