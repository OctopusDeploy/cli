package executor

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
)

// task type definitions
const TaskTypeCreateAccount = "createAccount"

// ProcessTasks iterates over the list of tasks and attempts to run them all.
// If everything goes well, a nil error will be returned.
// On the first failure, the error will be returned.
// TODO some kind of progress callback?
func ProcessTasks(clientFactory apiclient.ClientFactory, tasks []Task) error {
	for _, task := range tasks {
		switch task.Type {
		case TaskTypeCreateAccount:
			if err := accountCreate(clientFactory, task.Options); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Unhandled task CommandType %s", task.Type))
		}
	}
	return nil
}

// account type values match the UI
const AccountTypeUsernamePassword = "Username/Password"
const AccountTypeToken = "Token"

type TaskOptionsCreateAccount struct {
	Type           string   // REQUIRED. refer to AccountType constant strings
	Name           string   // REQUIRED.
	Description    string   // optional
	EnvironmentIds []string // optional. // TODO the user may have specified environment string names; the outer code should resolve them before building the TaskInput
	Options        any      // subtype-specific payload
}

type TaskOptionsCreateAccountUsernamePassword struct {
	Username string
	Password *core.SensitiveValue
}

type TaskOptionsCreateAccountToken struct {
	Token *core.SensitiveValue
}

func accountCreate(clientFactory apiclient.ClientFactory, input any) error {
	params, ok := input.(TaskOptionsCreateAccount)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsCreateAccount")
	}

	client, err := clientFactory.GetSpacedClient()
	if err != nil {
		return err
	}

	// TODO should we validate these here, or should we assume that the outer code has already validated them?
	// most of the lines of code here are validation
	accountName := params.Name
	if params.Name == "" {
		return errors.New("must specify account name")
	}

	var account accounts.IAccount = nil
	switch params.Type {
	// Note the Command Processor will have screened and converted any user input first,
	// so if we want to be nice and allow multiple options with the same meaning, that is the place to handle it,
	// rather than here
	case AccountTypeUsernamePassword:
		options, ok := params.Options.(TaskOptionsCreateAccountUsernamePassword)
		if !ok {
			return errors.New("Options must be TaskInputCreateAccountUsernamePassword")
		}

		p, err := accounts.NewUsernamePasswordAccount(accountName)
		if err != nil {
			return err
		}
		account = p

		if options.Username == "" {
			return errors.New("must specify username")
		}
		p.Username = options.Username

		if !options.Password.HasValue {
			return errors.New("must specify password")
		}
		p.Password = options.Password

	case AccountTypeToken:
		options, ok := params.Options.(TaskOptionsCreateAccountToken)
		if !ok {
			return errors.New("Options must be TaskInputCreateAccountUsernamePassword")
		}

		if !options.Token.HasValue {
			return errors.New("must specify token")
		}

		p, err := accounts.NewTokenAccount(accountName, options.Token)
		if err != nil {
			return err
		}
		account = p

		// TODO AWS, Azure, Google accounts etc

	default:
		return errors.New(fmt.Sprintf("Unhandled account type %s", params.Type))
	}

	// common
	if params.Description != "" {
		account.SetDescription(params.Description)
	}

	if len(params.EnvironmentIds) > 0 {
		account.SetEnvironmentIDs(params.EnvironmentIds)
	}

	_, err = client.Accounts.Add(account)
	return err
}
