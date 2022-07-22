//go:build integration

// see README section explaining how to run integration tests

package account_test

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/integrationtest"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAccountList(t *testing.T) {
	// setup
	octopusApiClient, err := integrationtest.GetApiClient("")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("default space", func(t *testing.T) {
		// setup
		newAccount, _ := accounts.NewUsernamePasswordAccount("user-pw-a")
		newAccount.SetUsername("user-a")
		newAccount.SetPassword(core.NewSensitiveValue("password-a"))
		accountRef1, err := octopusApiClient.Accounts.Add(newAccount)
		if err != nil {
			t.Fatal(err.Error())
		}

		newAccount2, _ := accounts.NewUsernamePasswordAccount("user-pw-b")
		newAccount2.SetUsername("user-b")
		newAccount2.SetPassword(core.NewSensitiveValue("password-b"))
		accountRef2, err := octopusApiClient.Accounts.Add(newAccount2)
		if err != nil {
			t.Fatal(err.Error())
		}

		accountID1 := accountRef1.GetID()
		accountID2 := accountRef2.GetID()

		defer func() {
			if err := octopusApiClient.Accounts.DeleteByID(accountRef2.GetID()); err != nil {
				t.Fatal(fmt.Sprintf("Abort! Error cleaning up test fixture data. %s", err))
			}
			if err := octopusApiClient.Accounts.DeleteByID(accountRef1.GetID()); err != nil {
				t.Fatal(fmt.Sprintf("Abort! Error cleaning up test fixture data. %s", err))
			}
		}()

		t.Run("--format basic", func(t *testing.T) {
			stdOut, stdErr, err := integrationtest.RunCli("Default", "account", "list", "--outputFormat=basic")
			if err != nil {
				t.Log(stdOut)
				t.Log(stdErr)
				t.Errorf(err.Error())
			} else {
				assert.Equal(t, heredoc.Doc(`
user-pw-a
user-pw-b
`), stdOut)
			}
		})

		t.Run("--format table", func(t *testing.T) {
			stdOut, stdErr, err := integrationtest.RunCli("Default", "account", "list", "--outputFormat=table")
			if err != nil {
				t.Log(stdOut)
				t.Log(stdErr)
				t.Errorf(err.Error())
			} else {
				assert.Equal(t, heredoc.Doc(`
NAME       TYPE
user-pw-a  UsernamePassword
user-pw-b  UsernamePassword
`), stdOut)
			}
		})

		t.Run("--format json", func(t *testing.T) {
			stdOut, stdErr, err := integrationtest.RunCli("Default", "account", "list", "--outputFormat=json")
			if err != nil {
				t.Log(stdOut)
				t.Log(stdErr)
				t.Errorf(err.Error())
			} else {
				// TODO| doing string comparison on JSON is awful and unreliable because JSON doesn't guarantee
				// TODO| dictionary order, and we will burn lots of time on fiddling around with whitespace.
				// TODO| Parse the text into a structure and assert on that instead
				expected := heredoc.Docf(`
[
  {
    "Id": "%s",
    "Name": "user-pw-a"
  },
  {
    "Id": "%s",
    "Name": "user-pw-b"
  }
]
`, accountID1, accountID2)
				assert.Equal(t, expected, stdOut)
			}
		})

	})
}
