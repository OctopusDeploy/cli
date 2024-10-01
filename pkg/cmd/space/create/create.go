package create

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/validation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/spf13/cobra"
)

const (
	FlagName        = "name"
	FlagDescription = "description"
	FlagTeam        = "team"
	FlagUser        = "user"
)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Teams       *flag.Flag[[]string]
	Users       *flag.Flag[[]string]
}

type CreateOptions struct {
	*cmd.Dependencies
	*CreateFlags
	GetAllSpacesCallback shared.GetAllSpacesCallback
	GetAllTeamsCallback  shared.GetAllTeamsCallback
	GetAllUsersCallback  shared.GetAllUsersCallback
}

func NewCreateOptions(f factory.Factory, flags *CreateFlags, c *cobra.Command) *CreateOptions {
	dependencies := cmd.NewSystemDependencies(f, c)
	client, err := f.GetSystemClient(apiclient.NewRequester(c))
	dependencies.Client = client // override the default space client
	if err != nil {
		panic(err)
	}
	return &CreateOptions{
		CreateFlags:          flags,
		Dependencies:         dependencies,
		GetAllSpacesCallback: func() ([]*spaces.Space, error) { return shared.GetAllSpaces(*client) },
		GetAllTeamsCallback:  func() ([]*teams.Team, error) { return shared.GetAllTeams(*client) },
		GetAllUsersCallback:  func() ([]*users.User, error) { return shared.GetAllUsers(*client) },
	}
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		Teams:       flag.New[[]string](FlagTeam, false),
		Users:       flag.New[[]string](FlagUser, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a space",
		Long:    "Create a space in Octopus Deploy",
		Example: heredoc.Docf("$ %s space create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(f, createFlags, c)

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the space")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the space")
	flags.StringArrayVarP(&createFlags.Teams.Value, createFlags.Teams.Name, "t", nil, "The teams to manage the space (can be specified multiple times)")
	flags.StringArrayVarP(&createFlags.Users.Value, createFlags.Users.Name, "u", nil, "The users to manage the space (can be specified multiple times)")

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}
	space := spaces.NewSpace(opts.Name.Value)

	allTeams, err := opts.Client.Teams.GetAll()
	if err != nil {
		return err
	}

	allUsers, err := opts.Client.Users.GetAll()
	if err != nil {
		return err
	}

	for _, team := range opts.Teams.Value {
		team, err := findTeam(allTeams, team)
		if err != nil {
			return err
		}

		space.SpaceManagersTeams = append(space.SpaceManagersTeams, team.ID)
	}

	for _, user := range opts.Users.Value {
		user, err := findUser(allUsers, user)
		if err != nil {
			return err
		}

		space.SpaceManagersTeamMembers = append(space.SpaceManagersTeamMembers, user.GetID())
	}

	space.Description = opts.Description.Value

	createdSpace, err := opts.Client.Spaces.Add(space)
	if err != nil {
		return err
	}

	fmt.Printf("%s The space, \"%s\" %s was created successfully.\n", output.Green("✔"), createdSpace.Name, output.Dimf("(%s)", createdSpace.ID))

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Description, opts.Teams, opts.Users)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func findTeam(allTeams []*teams.Team, identifier string) (*teams.Team, error) {
	for _, team := range allTeams {
		if strings.EqualFold(identifier, team.ID) || strings.EqualFold(identifier, team.Name) {
			return team, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Cannot find team '%s'", identifier))
}

func findUser(allUsers []*users.User, identifier string) (*users.User, error) {
	for _, user := range allUsers {
		if strings.EqualFold(identifier, user.ID) || strings.EqualFold(identifier, user.Username) || strings.EqualFold(identifier, user.DisplayName) {
			return user, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Cannot find user '%s'", identifier))
}

func selectTeams(ask question.Asker, getAllTeamsCallback shared.GetAllTeamsCallback, existingSpaces []*spaces.Space, message string) ([]*teams.Team, error) {
	systemTeams, err := getAllTeamsCallback()
	if err != nil {
		return []*teams.Team{}, err
	}

	return question.MultiSelectMap(ask, message, systemTeams, func(team *teams.Team) string {
		if len(team.SpaceID) == 0 {
			return fmt.Sprintf("%s %s", team.Name, output.Dim("(System Team)"))
		}
		for _, existingSpace := range existingSpaces {
			if team.SpaceID == existingSpace.ID {
				return fmt.Sprintf("%s %s", team.Name, output.Dimf("(%s)", existingSpace.Name))
			}
		}
		return ""
	}, false)
}

func selectUsers(ask question.Asker, getAllUsersCallback shared.GetAllUsersCallback, message string) ([]*users.User, error) {
	existingUsers, err := getAllUsersCallback()
	if err != nil {
		return nil, err
	}

	return question.MultiSelectMap(ask, message, existingUsers, func(existingUser *users.User) string {
		return fmt.Sprintf("%s %s", existingUser.DisplayName, output.Dimf("(%s)", existingUser.Username))
	}, false)
}

func PromptMissing(opts *CreateOptions) error {
	existingSpaces, err := opts.GetAllSpacesCallback()
	if err != nil {
		return err
	}

	spaceNames := util.SliceTransform(existingSpaces, func(s *spaces.Space) string { return s.Name })
	if opts.Name.Value == "" {
		err = opts.Ask(&survey.Input{
			Help:    "The name of the space being created.",
			Message: "Name",
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(20),
			survey.MinLength(1),
			survey.Required,
			validation.NotEquals(spaceNames, "a space with this name already exists"),
		)))
		if err != nil {
			return err
		}
	}

	err = question.AskDescription(opts.Ask, "", "space", &opts.Description.Value)
	if err != nil {
		return err
	}

	if len(opts.Teams.Value) == 0 {
		selectedTeams, err := selectTeams(opts.Ask, opts.GetAllTeamsCallback, existingSpaces, "Select one or more teams to manage this space:")
		if err != nil {
			return err
		}

		for _, team := range selectedTeams {
			opts.Teams.Value = append(opts.Teams.Value, team.Name)
		}
	}

	if len(opts.Users.Value) == 0 {
		selectedUsers, err := selectUsers(opts.Ask, opts.GetAllUsersCallback, "Select one or more users to manage this space:")
		if err != nil {
			return err
		}

		for _, user := range selectedUsers {
			opts.Users.Value = append(opts.Users.Value, user.Username)
		}
	}

	return nil
}
