package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/validation"
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

	existingSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	spaceNames := []string{}
	for _, existingSpace := range existingSpaces {
		spaceNames = append(spaceNames, existingSpace.Name)
	}

	var name string
	err = question.AskOne(&survey.Input{
		Help:    "The name of the space being created.",
		Message: "Name",
	}, &name, survey.WithValidator(survey.ComposeValidators(
		survey.MaxLength(20),
		survey.MinLength(1),
		survey.Required,
		validation.NotEquals(spaceNames, "a space with this name already exists"),
	)))
	if err != nil {
		return err
	}
	space := spaces.NewSpace(name)

	teams, err := selectTeams(client, existingSpaces, "Select one or more teams to manage this space:")
	if err != nil {
		return err
	}

	for _, team := range teams {
		space.SpaceManagersTeams = append(space.SpaceManagersTeams, team.ID)
	}

	users, err := selectUsers(client, "Select one or more users to manage this space:")
	if err != nil {
		return err
	}

	for _, user := range users {
		space.SpaceManagersTeamMembers = append(space.SpaceManagersTeams, user.ID)
	}

	createdSpace, err := client.Spaces.Add(space)
	if err != nil {
		return err
	}

	fmt.Printf("%s The space, \"%s\" %s was created successfully.\n", output.Green("✔"), createdSpace.Name, output.Dimf("(%s)", createdSpace.ID))
	return nil
}

func selectTeams(client *client.Client, existingSpaces []*spaces.Space, message string) ([]*teams.Team, error) {
	selectedTeams := []*teams.Team{}

	systemTeams, err := client.Teams.Get(teams.TeamsQuery{
		IncludeSystem: true,
	})
	if err != nil {
		return selectedTeams, err
	}

	err = question.MultiSelect(message, systemTeams.Items, func(team *teams.Team) string {
		if len(team.SpaceID) == 0 {
			return fmt.Sprintf("%s %s", team.Name, output.Dim("(System Team)"))
		}
		for _, existingSpace := range existingSpaces {
			if team.SpaceID == existingSpace.ID {
				return fmt.Sprintf("%s %s", team.Name, output.Dimf("(%s)", existingSpace.Name))
			}
		}
		return ""
	}, &selectedTeams)
	return selectedTeams, err
}

func selectUsers(client *client.Client, message string) ([]*users.User, error) {
	selectedUsers := []*users.User{}

	existingUsers, err := client.Users.GetAll()
	if err != nil {
		return selectedUsers, err
	}

	err = question.MultiSelect(message, existingUsers, func(existingUser *users.User) string {
		return fmt.Sprintf("%s %s", existingUser.DisplayName, output.Dimf("(%s)", existingUser.Username))
	}, &selectedUsers)
	return selectedUsers, err
}
