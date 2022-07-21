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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
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

	allSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	_, err = askSpaceName(allSpaces)
	if err != nil {
		return err
	}

	_, err = selectTeams(client, allSpaces, "Select one or more teams to manage this space:")
	if err != nil {
		return err
	}

	_, err = selectUsers(client, "Select one or more users to manage this space:")
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

func askSpaceName(existingSpaces []*spaces.Space) (string, error) {
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

				for _, existingSpace := range existingSpaces {
					if name == existingSpace.Name {
						return errors.New("a space with this name already exists; please specify a unique name")
					}
				}
			}
			return nil
		},
	}

	var name string
	err := survey.Ask([]*survey.Question{nameQuestion}, &name)
	return name, err
}

func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func selectTeams(client *client.Client, existingSpaces []*spaces.Space, message string) ([]*teams.Team, error) {
	selectedTeams := []*teams.Team{}

	systemTeams, err := client.Teams.Get(teams.TeamsQuery{
		IncludeSystem: false,
	})
	if err != nil {
		return selectedTeams, err
	}

	teamNames := map[string]string{}
	for _, team := range systemTeams.Items {
		if len(team.SpaceID) == 0 {
			teamNames[fmt.Sprintf("%s %s", team.Name, output.Dim("(System Team)"))] = team.ID
		} else {
			for _, existingSpace := range existingSpaces {
				if team.SpaceID == existingSpace.ID {
					teamNames[fmt.Sprintf("%s %s", team.Name, output.Dimf("(%s)", existingSpace.Name))] = team.GetID()
				}
			}
		}
	}

	selectedNames := []string{}
	err = survey.Ask([]*survey.Question{
		{
			Name: "teams",
			Prompt: &survey.MultiSelect{
				Message: message,
				Options: getKeys(teamNames),
			},
		},
	}, &selectedNames)

	for _, name := range selectedNames {
		for _, team := range systemTeams.Items {
			if team.ID == teamNames[name] {
				selectedTeams = append(selectedTeams, team)
				break
			}
		}
	}

	return selectedTeams, err
}

func selectUsers(client *client.Client, message string) ([]*users.User, error) {
	selectedUsers := []*users.User{}

	existingUsers, err := client.Users.GetAll()
	if err != nil {
		return selectedUsers, err
	}

	userDisplayNames := map[string]string{}
	for _, existingUser := range existingUsers {
		userDisplayNames[fmt.Sprintf("%s %s", existingUser.DisplayName, output.Dimf("(%s)", existingUser.Username))] = existingUser.GetID()
	}

	selectedNames := []string{}
	err = survey.Ask([]*survey.Question{
		{
			Name: "users",
			Prompt: &survey.MultiSelect{
				Message: message,
				Options: getKeys(userDisplayNames),
			},
		},
	}, &selectedNames)

	for _, name := range selectedNames {
		for _, existingUser := range existingUsers {
			if existingUser.ID == userDisplayNames[name] {
				selectedUsers = append(selectedUsers, existingUser)
				break
			}
		}
	}

	return selectedUsers, err
}
