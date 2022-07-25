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
		switch task.CommandType {
		case "accountCreate":
			if err := accountCreate(clientFactory, task.Inputs); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Unhandled task CommandType %s", task.CommandType))
		}
	}
	return nil
}

// attribute keys
const NameKey = "name"
const TypeKey = "type"
const DescriptionKey = "description"
const AccountUsernameKey = "username"
const AccountPasswordKey = "password"
const AccountTokenKey = "token"
const AccountEnvironmentIDsKey = "environmentids"

// account type values match the UI
const AccountUsernamePasswordType = "Username/Password"
const AccountTokenType = "Token"

func accountCreate(clientFactory apiclient.ClientFactory, inputs map[string]any) error {
	client, err := clientFactory.GetSpacedClient()
	if err != nil {
		return err
	}

	accountName, ok := inputs[NameKey].(string)
	if !ok || accountName == "" {
		return errors.New("must specify account name")
	}

	// TODO set of environments

	var account accounts.IAccount = nil

	accountType, ok := inputs[TypeKey].(string)
	if !ok {
		accountType = ""
	}
	switch accountType {
	// Note the Command Processor will have screened and converted any user input first,
	// so if we want to be nice and allow multiple options with the same meaning, that is the place to handle it,
	// rather than here
	case AccountUsernamePasswordType:
		p, err := accounts.NewUsernamePasswordAccount(accountName)
		if err != nil {
			return err
		}
		account = p

		username, ok := inputs[AccountUsernameKey].(string)
		if !ok || username == "" {
			return errors.New("must specify username")
		}
		p.Username = username

		password, ok := inputs[AccountPasswordKey].(string)
		if !ok || password == "" {
			return errors.New("must specify password")
		}
		p.SetPassword(core.NewSensitiveValue(password))
	case AccountTokenType:
		token, ok := inputs[AccountTokenKey].(string)
		if !ok || token == "" {
			return errors.New("must specify token")
		}

		p, err := accounts.NewTokenAccount(accountName, core.NewSensitiveValue(token))
		if err != nil {
			return err
		}
		account = p
	default:
		return errors.New(fmt.Sprintf("Unhandled account type %s", accountType))
	}

	// common
	description, ok := inputs[DescriptionKey].(string)
	if ok && description != "" {
		account.SetDescription("description")
	}

	environmentIDs, ok := inputs[AccountEnvironmentIDsKey].([]string)
	if ok && len(environmentIDs) > 0 {
		account.SetEnvironmentIDs(environmentIDs)
	}

	_, err = client.Accounts.Add(account)
	return err
}
