package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
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

func createRun(f factory.Factory, _ io.Writer) error {
	systemClient, err := f.GetSystemClient()
	if err != nil {
		return err
	}

	existingSpaces, err := systemClient.Spaces.GetAll()
	if err != nil {
		return err
	}

	spaceNames := []string{}
	for _, existingSpace := range existingSpaces {
		spaceNames = append(spaceNames, existingSpace.Name)
	}

	var name string
	err = f.Ask(&survey.Input{
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

	teams, err := selectTeams(f.Ask, systemClient, existingSpaces, "Select one or more teams to manage this space:")
	if err != nil {
		return err
	}

	for _, team := range teams {
		space.SpaceManagersTeams = append(space.SpaceManagersTeams, team.ID)
	}

	users, err := selectUsers(f.Ask, systemClient, "Select one or more users to manage this space:")
	if err != nil {
		return err
	}

	for _, user := range users {
		space.SpaceManagersTeamMembers = append(space.SpaceManagersTeams, user.ID)
	}

	createdSpace, err := systemClient.Spaces.Add(space)
	if err != nil {
		return err
	}

	fmt.Printf("%s The space, \"%s\" %s was created successfully.\n", output.Green("✔"), createdSpace.Name, output.Dimf("(%s)", createdSpace.ID))
	return nil
}

func selectTeams(ask question.Asker, client *client.Client, existingSpaces []*spaces.Space, message string) ([]*teams.Team, error) {
	systemTeams, err := client.Teams.Get(teams.TeamsQuery{
		IncludeSystem: true,
	})
	if err != nil {
		return []*teams.Team{}, err
	}

	return question.MultiSelectMap(ask, message, systemTeams.Items, func(team *teams.Team) string {
		if len(team.SpaceID) == 0 {
			return fmt.Sprintf("%s %s", team.Name, output.Dim("(System Team)"))
		}
		for _, existingSpace := range existingSpaces {
			if team.SpaceID == existingSpace.ID {
				return fmt.Sprintf("%s %s", team.Name, output.Dimf("(%s)", existingSpace.Name))
			}
		}
		return ""
	})
}

func selectUsers(ask question.Asker, client *client.Client, message string) ([]*users.User, error) {
	selectedUsers := []*users.User{}

	existingUsers, err := client.Users.GetAll()
	if err != nil {
		return selectedUsers, err
	}

	return question.MultiSelectMap(ask, message, existingUsers, func(existingUser *users.User) string {
		return fmt.Sprintf("%s %s", existingUser.DisplayName, output.Dimf("(%s)", existingUser.Username))
	})
}
