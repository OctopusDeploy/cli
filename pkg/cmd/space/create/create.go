package create

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"io"
	"strings"

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
	*CreateFlags
	Client   *client.Client
	Out      io.Writer
	Ask      question.Asker
	NoPrompt bool
}

func NewCreateOptions(f factory.Factory, cmd *cobra.Command, flags *CreateFlags) *CreateOptions {
	client, err := f.GetSystemClient()
	if err != nil {
		panic(err)
	}
	return &CreateOptions{
		CreateFlags: flags,
		Ask:         f.Ask,
		Client:      client,
		Out:         cmd.OutOrStdout(),
		NoPrompt:    !f.IsPromptEnabled(),
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
		Use:   "create",
		Short: "Creates a space in an instance of Octopus Deploy",
		Long:  "Creates a space in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := NewCreateOptions(f, cmd, createFlags)

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the space")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the space")
	flags.StringSliceVarP(&createFlags.Teams.Value, createFlags.Teams.Name, "t", nil, "The teams to manage the space (can be specified multiple times)")
	flags.StringSliceVarP(&createFlags.Users.Value, createFlags.Users.Name, "u", nil, "The users to manage the space (can be specified multiple times)")

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

	createdSpace, err := opts.Client.Spaces.Add(space)
	if err != nil {
		return err
	}

	fmt.Printf("%s The space, \"%s\" %s was created successfully.\n", output.Green("✔"), createdSpace.Name, output.Dimf("(%s)", createdSpace.ID))
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
	}, false)
}

func selectUsers(ask question.Asker, client *client.Client, message string) ([]*users.User, error) {
	existingUsers, err := client.Users.GetAll()
	if err != nil {
		return nil, err
	}

	return question.MultiSelectMap(ask, message, existingUsers, func(existingUser *users.User) string {
		return fmt.Sprintf("%s %s", existingUser.DisplayName, output.Dimf("(%s)", existingUser.Username))
	}, false)
}

func PromptMissing(opts *CreateOptions) error {
	existingSpaces, err := opts.Client.Spaces.GetAll()
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

	if opts.Description.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Description",
			Help:    "A short, memorable, description for this space.",
		}, &opts.Description.Value); err != nil {
			return err
		}
	}

	if len(opts.Teams.Value) == 0 {
		selectedTeams, err := selectTeams(opts.Ask, opts.Client, existingSpaces, "Select one or more teams to manage this space:")
		if err != nil {
			return err
		}

		for _, team := range selectedTeams {
			opts.Teams.Value = append(opts.Teams.Value, team.ID)
		}
	}

	if len(opts.Users.Value) == 0 {
		selectedUsers, err := selectUsers(opts.Ask, opts.Client, "Select one or more users to manage this space:")
		if err != nil {
			return err
		}

		for _, user := range selectedUsers {
			opts.Users.Value = append(opts.Users.Value, user.ID)
		}
	}

	return nil
}
