package create_test

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/space/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()
	flags.Name.Value = "The Final Frontier"
	flags.Description.Value = "Where no person has gone before"
	flags.Teams.Value = []string{"Crew"}
	flags.Users.Value = []string{"James T. Kirk"}

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		GetAllSpacesCallback: func() ([]*spaces.Space, error) {
			return []*spaces.Space{
				spaces.NewSpace("Explored space")}, nil
		},
	}
	create.PromptMissing(opts)
	checkRemainingPrompts()
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	captain := users.NewUser("james.kirk@enterprise.alphaquad.com", "James T. Kirk")
	captain.ID = "Users-1"
	vulcan := users.NewUser("spock@enterprise.alphaquad.com", "Spock")
	vulcan.ID = "Users-2"
	bridgeTeam := teams.NewTeam("Bridge crew")
	bridgeTeam.ID = "Teams-1"
	engineeringTeam := teams.NewTeam("Engineering")
	engineeringTeam.ID = "Teams-2"

	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "The name of the space being created.", "Cray cray space"),
		testutil.NewInputPrompt("Description", "A short, memorable, description for this space.", "Crazy description"),
		testutil.NewMultiSelectPrompt("Select one or more teams to manage this space:", "", []string{formatTeam(bridgeTeam), formatTeam(engineeringTeam)}, []string{formatTeam(bridgeTeam)}),
		testutil.NewMultiSelectPrompt("Select one or more users to manage this space:", "", []string{formatUser(captain), formatUser(vulcan)}, []string{formatUser(vulcan)}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := create.NewCreateFlags()

	opts := &create.CreateOptions{
		CreateFlags:  flags,
		Dependencies: &cmd.Dependencies{Ask: asker},
		GetAllSpacesCallback: func() ([]*spaces.Space, error) {
			return []*spaces.Space{
				spaces.NewSpace("Explored space")}, nil
		},
		GetAllTeamsCallback: func() ([]*teams.Team, error) {

			return []*teams.Team{
				bridgeTeam,
				engineeringTeam,
			}, nil
		},
		GetAllUsersCallback: func() ([]*users.User, error) { return []*users.User{captain, vulcan}, nil },
	}
	create.PromptMissing(opts)
	checkRemainingPrompts()
	assert.Equal(t, "Cray cray space", flags.Name.Value)
	assert.Equal(t, "Crazy description", flags.Description.Value)
	assert.Equal(t, []string{bridgeTeam.Name}, flags.Teams.Value)
	assert.Equal(t, []string{vulcan.Username}, flags.Users.Value)
}

func formatUser(user *users.User) string {
	return fmt.Sprintf("%s (%s)", user.DisplayName, output.Dimf(user.Username))
}

func formatTeam(team *teams.Team) string {
	return fmt.Sprintf("%s %s", team.Name, output.Dimf("(System Team)"))
}
