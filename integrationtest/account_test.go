package integrationtest

import (
	"encoding/json"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAccountList(t *testing.T) {
	// setup
	octopusApiClient, err := GetApiClient("")

	if err != nil {
		t.Fatal(err)
	}

	t.Run("default space", func(t *testing.T) {
		cleanupHelper := NewCleanupHelper()
		defer cleanupHelper.Run(t)

		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("user-pw-a")
		newAccount.SetUsername("user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := octopusApiClient.Accounts.Add(newAccount)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error {
			return octopusApiClient.Accounts.DeleteByID(accountRef1.GetID())
		})

		newAccount2, _ := accounts.NewUsernamePasswordAccount("user-pw-b")
		newAccount2.SetUsername("user-b")
		newAccount2.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := octopusApiClient.Accounts.Add(newAccount2)
		if !EnsureSuccess(t, err) {
			return
		}
		cleanupHelper.AddFailable(func() error {
			return octopusApiClient.Accounts.DeleteByID(accountRef2.GetID())
		})

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
			type IdAndName struct {
				Id   string
				Name string
			}
			var results []IdAndName
			err = json.Unmarshal(stdOut, &results)
			if !EnsureSuccess(t, err, string(stdOut), string(stdErr)) {
				return
			}

			expected := []IdAndName{
				{Id: accountRef1.GetID(), Name: "user-pw-a"},
				{Id: accountRef2.GetID(), Name: "user-pw-b"},
			}
			assert.Equal(t, expected, results)
		})
	})
}
