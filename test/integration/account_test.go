package integration_test

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/stretchr/testify/assert"
)

func TestAccountList(t *testing.T) {
	// setup
	systemApiClient, err := integration.GetApiClient("")
	if !testutil.AssertSuccess(t, err) {
		return
	}

	t.Run("default space", func(t *testing.T) {
		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("user-pw-a")
		newAccount.SetUsername("user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := systemApiClient.Accounts.Add(newAccount)
		if !testutil.AssertSuccess(t, err) {
			return
		}
		t.Cleanup(func() { require.Nil(t, systemApiClient.Accounts.DeleteByID(accountRef1.GetID())) })

		newAccount2, _ := accounts.NewUsernamePasswordAccount("user-pw-b")
		newAccount2.SetUsername("user-b")
		newAccount2.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := systemApiClient.Accounts.Add(newAccount2)
		if !testutil.AssertSuccess(t, err) {
			return
		}
		t.Cleanup(func() { assert.Nil(t, systemApiClient.Accounts.DeleteByID(accountRef2.GetID())) })

		t.Run("--format basic", func(t *testing.T) {
			stdOut, stdErr, err := integration.RunCli("Default", "account", "list", "--outputFormat=basic")
			if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
				return
			}
			assert.Equal(t, heredoc.Doc(`
user-pw-a
user-pw-b
`), stdOut)
		})

		t.Run("--format table", func(t *testing.T) {
			stdOut, stdErr, err := integration.RunCli("Default", "account", "list", "--outputFormat=table")
			if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
				return
			}

			assert.Equal(t, heredoc.Doc(`
NAME       TYPE
user-pw-a  Username/Password
user-pw-b  Username/Password
`), stdOut)
		})

		t.Run("--format json", func(t *testing.T) {
			stdOut, stdErr, err := integration.RunCliRawOutput("Default", "account", "list", "--outputFormat=json")
			if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
				return
			}
			type AccountSummary struct {
				Id   string
				Name string
				Type string
			}
			var results []AccountSummary
			err = json.Unmarshal(stdOut, &results)
			if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
				return
			}

			expected := []AccountSummary{
				{Id: accountRef1.GetID(), Name: "user-pw-a", Type: "UsernamePassword"},
				{Id: accountRef2.GetID(), Name: "user-pw-b", Type: "UsernamePassword"},
			}
			assert.Equal(t, expected, results)
		})
	})

	t.Run("different space ", func(t *testing.T) {
		systemTeams, err := systemApiClient.Teams.Get(teams.TeamsQuery{
			IncludeSystem: true,
		})

		space := spaces.NewSpace("my-new-space")

		for _, team := range systemTeams.Items {
			space.SpaceManagersTeams = append(space.SpaceManagersTeams, team.GetID())
		}

		space, err = systemApiClient.Spaces.Add(space)
		if !testutil.AssertSuccess(t, err) {
			return
		}
		t.Cleanup(func() {
			space.TaskQueueStopped = true // make sure we can delete it at the end, we're not actually doing any tasks here
			_, err = systemApiClient.Spaces.Update(space)
			require.Nil(t, err)

			err = systemApiClient.Spaces.DeleteByID(space.GetID())
			require.Nil(t, err)
		})

		spacedApiClient, err := integration.GetApiClient(space.GetID())
		if !testutil.AssertSuccess(t, err) {
			return
		}

		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("spaced-user-pw-a")
		newAccount.SetUsername("spaced-user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := spacedApiClient.Accounts.Add(newAccount)
		if !testutil.AssertSuccess(t, err) {
			return
		}
		t.Cleanup(func() {
			require.Nil(t, spacedApiClient.Accounts.DeleteByID(accountRef1.GetID()))
		})

		newAccountDifferentSpace, _ := accounts.NewUsernamePasswordAccount("defspace-user-pw-b")
		newAccountDifferentSpace.SetUsername("defspace-user-b")
		newAccountDifferentSpace.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := systemApiClient.Accounts.Add(newAccountDifferentSpace)
		if !testutil.AssertSuccess(t, err) {
			return
		}
		t.Cleanup(func() { require.Nil(t, systemApiClient.Accounts.DeleteByID(accountRef2.GetID())) })

		t.Run("--format basic", func(t *testing.T) {
			stdOut, stdErr, err := integration.RunCli("my-new-space", "account", "list", "--outputFormat=basic")
			if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
				return
			}
			// note default spaced item is NOT shown
			assert.Equal(t, heredoc.Doc(`
	spaced-user-pw-a
	`), stdOut)
		})

		//tests for JSON and Table are redundant here because the CLI is calling the same API's in the server
		//that we have just tested. The only difference is output format, which is tested elsewhere
	})
}
