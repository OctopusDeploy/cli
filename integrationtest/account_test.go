package integrationtest

import (
	"encoding/json"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAccountList(t *testing.T) {
	// setup
	systemApiClient, err := GetApiClient("")
	if !EnsureSuccess(t, err) {
		return
	}

	t.Run("default space", func(t *testing.T) {
		cleanupHelper := NewCleanupHelper()
		defer cleanupHelper.Run(t)

		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("user-pw-a")
		newAccount.SetUsername("user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := systemApiClient.Accounts.Add(newAccount)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error { return systemApiClient.Accounts.DeleteByID(accountRef1.GetID()) })

		newAccount2, _ := accounts.NewUsernamePasswordAccount("user-pw-b")
		newAccount2.SetUsername("user-b")
		newAccount2.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := systemApiClient.Accounts.Add(newAccount2)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error { return systemApiClient.Accounts.DeleteByID(accountRef2.GetID()) })

		t.Run("--format basic", func(t *testing.T) {
			stdOut, stdErr, err := RunCli("Default", "account", "list", "--outputFormat=basic")
			if !EnsureSuccess(t, err, stdOut, stdErr) {
				return
			}
			assert.Equal(t, heredoc.Doc(`
user-pw-a
user-pw-b
`), stdOut)
		})

		t.Run("--format table", func(t *testing.T) {
			stdOut, stdErr, err := RunCli("Default", "account", "list", "--outputFormat=table")
			if !EnsureSuccess(t, err, stdOut, stdErr) {
				return
			}

			assert.Equal(t, heredoc.Doc(`
NAME       TYPE
user-pw-a  UsernamePassword
user-pw-b  UsernamePassword
`), stdOut)
		})

		t.Run("--format json", func(t *testing.T) {
			stdOut, stdErr, err := RunCliRawOutput("Default", "account", "list", "--outputFormat=json")
			if !EnsureSuccess(t, err, stdOut, stdErr) {
				return
			}
			type AccountSummary struct {
				Id   string
				Name string
				Type string
			}
			var results []AccountSummary
			err = json.Unmarshal(stdOut, &results)
			if !EnsureSuccess(t, err, string(stdOut), string(stdErr)) {
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
		cleanupHelper := NewCleanupHelper()
		defer cleanupHelper.Run(t)

		systemTeams, err := systemApiClient.Teams.Get(teams.TeamsQuery{
			IncludeSystem: true,
		})

		myNewSpace := spaces.NewSpace("my-new-space")

		for _, team := range systemTeams.Items {
			myNewSpace.SpaceManagersTeams = append(myNewSpace.SpaceManagersTeams, team.GetID())
		}

		myNewSpace, err = systemApiClient.Spaces.Add(myNewSpace)
		cleanupHelper.AddFailable(func() error {
			myNewSpace.TaskQueueStopped = true // make sure we can delete it at the end, we're not actually doing any tasks here

			var result *multierror.Error
			if _, err := systemApiClient.Spaces.Update(myNewSpace); err != nil {
				result = multierror.Append(err)
			}
			if err = systemApiClient.Spaces.DeleteByID(myNewSpace.GetID()); err != nil {
				result = multierror.Append(err)
			}
			return result.ErrorOrNil()
		})

		spacedApiClient, err := GetApiClient(myNewSpace.GetID())
		if !EnsureSuccess(t, err) {
			return
		}

		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("spaced-user-pw-a")
		newAccount.SetUsername("spaced-user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := spacedApiClient.Accounts.Add(newAccount)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error {
			return spacedApiClient.Accounts.DeleteByID(accountRef1.GetID())
		})

		newAccountDifferentSpace, _ := accounts.NewUsernamePasswordAccount("defspace-user-pw-b")
		newAccountDifferentSpace.SetUsername("defspace-user-b")
		newAccountDifferentSpace.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := systemApiClient.Accounts.Add(newAccountDifferentSpace)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error { return systemApiClient.Accounts.DeleteByID(accountRef2.GetID()) })

		t.Run("--format basic", func(t *testing.T) {
			stdOut, stdErr, err := RunCli("my-new-space", "account", "list", "--outputFormat=basic")
			if !EnsureSuccess(t, err, stdOut, stdErr) {
				return
			}
			// note default spaced item is NOT shown
			assert.Equal(t, heredoc.Doc(`
spaced-user-pw-a
`), stdOut)
		})

		// tests for JSON and Table are redundant here because the CLI is calling the same API's in the server
		// that we have just tested. The only difference is output format, which is tested elsewhere
	})
}
