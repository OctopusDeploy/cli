package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

type ViewOptions struct {
	Client   *client.Client
	Host     string
	Selector string
	Command  *cobra.Command
}

func NewCmdView(f factory.Factory) *cobra.Command {
	opts := &ViewOptions{}

	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a space",
		Long:  "View a space in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s space view 'Pattern - Blue-Green'
			$ %[1]s space view Spaces-302
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts.Client = client
			opts.Host = f.GetCurrentHost()
			opts.Selector = args[0]
			opts.Command = cmd

			return viewRun(opts)
		},
	}

	return cmd
}

func viewRun(opts *ViewOptions) error {
	space, err := opts.Client.Spaces.GetByIDOrName(opts.Selector)
	if err != nil {
		return err
	}

	host := opts.Host
	return output.PrintResource(space, opts.Command, output.Mappers[*spaces.Space]{
		Json: func(item *spaces.Space) any {
			return SpaceAsJson{
				Id:          item.GetID(),
				Name:        item.Name,
				Description: item.Description,
				TaskQueue:   getTaskQueueStatus(item),
				WebUrl:      generateWebUrl(host, item),
			}
		},
		Table: output.TableDefinition[*spaces.Space]{
			Header: []string{"NAME", "DESCRIPTION", "TASK QUEUE", "WEB URL"},
			Row: func(item *spaces.Space) []string {
				description := item.Description
				if description == "" {
					description = constants.NoDescription
				}

				return []string{output.Bold(item.Name), description, getTaskQueueStatusColored(item), output.Blue(generateWebUrl(host, item))}
			},
		},
		Basic: func(item *spaces.Space) string {
			return formatSpaceForBasic(host, item)
		},
	})
}

type SpaceAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	TaskQueue   string `json:"TaskQueue"`
	WebUrl      string `json:"WebUrl"`
}

func getTaskQueueStatus(space *spaces.Space) string {
	if space.TaskQueueStopped {
		return "Stopped"
	}
	return "Running"
}

func getTaskQueueStatusColored(space *spaces.Space) string {
	if space.TaskQueueStopped {
		return output.Yellow("Stopped")
	}
	return output.Green("Running")
}

func generateWebUrl(host string, space *spaces.Space) string {
	link := strings.Split(space.Links["Web"], "/app#/")
	webLink := "/app#/configuration/" + link[1]
	return host + webLink
}

func formatSpaceForBasic(host string, space *spaces.Space) string {
	var result strings.Builder
	
	// header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(space.Name), output.Dimf("(%s)", space.GetID())))

	// metadata
	if len(space.Description) == 0 {
		result.WriteString(fmt.Sprintf("%s\n", output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintf("%s\n", output.Dim(space.Description)))
	}

	// task processing
	result.WriteString(fmt.Sprintf("Task Processing Status: %s\n", getTaskQueueStatusColored(space)))

	// footer
	result.WriteString(fmt.Sprintf("View this space in Octopus Deploy: %s", output.Blue(generateWebUrl(host, space))))

	return result.String()
}
