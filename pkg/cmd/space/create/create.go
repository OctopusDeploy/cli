package create

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a space in an instance of Octopus Deploy",
		Long:  "Creates a space in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createRun(f, cmd.OutOrStdout())
		},
	}

	return cmd
}

func createRun(f apiclient.ClientFactory, w io.Writer) error {
	client, err := f.Get(false)
	if err != nil {
		return err
	}

	_, err = askSpaceName(client)
	if err != nil {
		return err
	}

	_, err = selectSpaceManagers(client)
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

func askSpaceName(client *client.Client) (string, error) {
	nameQuestion := &survey.Question{
		Name: "name",
		Prompt: &survey.Input{
			Help:    "The name of the new space to be created. This name must be unique and cannot exceed 20 characters.",
			Message: "Name",
		},
		Validate: func(val interface{}) error {
			if s, ok := val.(string); ok {
				name := strings.TrimSpace(s)
				if len(name) <= 0 {
					return errors.New("name is required")
				}
				if len(name) > 20 {
					return errors.New("name cannot exceed 20 characters")
				}

				space, err := client.Spaces.GetByName(name)
				if err != nil {
					if apiError, ok := err.(*core.APIError); ok {
						if apiError.StatusCode != 404 {
							return err
						}
					}
				}

				if space != nil {
					return errors.New("a space with this name already exists; please specify a unique name")
				}
			}
			return nil
		},
	}

	var name string
	err := survey.Ask([]*survey.Question{nameQuestion}, &name)
	return name, err
}

func selectSpaceManagers(client *client.Client) ([]string, error) {
	spaceManagers := []string{}

	teams, err := client.Teams.GetAll()
	if err != nil {
		return spaceManagers, err
	}

	teamNames := []string{}
	for _, team := range teams {
		teamNames = append(teamNames, fmt.Sprintf("%s (%s)", team.Name, team.SpaceID))
	}

	questions := []*survey.Question{
		{
			Name: "teams",
			Prompt: &survey.MultiSelect{
				Message: "Select one or more teams to manage this space:",
				Options: teamNames,
			},
		},
	}

	err = survey.Ask(questions, &spaceManagers)
	return spaceManagers, err
}
