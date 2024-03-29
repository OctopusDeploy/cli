package delete

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var skipConfirmation bool
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete an account",
		Long:    "Delete an account in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s account delete
			$ %[1]s account rm
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deleteRun(f, cmd)
			}

			itemIDOrName := args[0]

			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))

			if err != nil {
				return err
			}

			// SDK doesn't have accounts.GetByIDOrName so we emulate it here
			foundAccounts, err := client.Accounts.Get(accounts.AccountsQuery{
				// TODO we can't lookup by ID yet because the server doesn't support it
				PartialName: itemIDOrName,
			})
			if err != nil {
				return err
			}
			// need exact match
			var itemToDelete accounts.IAccount
			for _, act := range foundAccounts.Items {
				if act.GetName() == itemIDOrName {
					itemToDelete = act
					break
				}
			}
			if itemToDelete == nil {
				return fmt.Errorf("cannot find an account with name or ID of '%s'", itemIDOrName)
			}

			if !skipConfirmation { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, "space", itemToDelete.GetName(), itemToDelete.GetID(), func() error {
					return delete(client, itemToDelete)
				})
			}

			return delete(client, itemToDelete)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &skipConfirmation, "account")

	return cmd
}

func deleteRun(f factory.Factory, cmd *cobra.Command) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	existingAccounts, err := client.Accounts.GetAll()
	if err != nil {
		return err
	}

	accountToDelete, err := selectors.ByName(f.Ask, existingAccounts, "Select the account you wish to delete:")
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(f.Ask, "account", accountToDelete.GetName(), accountToDelete.GetID(), func() error {
		return delete(client, accountToDelete)
	})
}

func delete(client *client.Client, accountToDelete accounts.IAccount) error {
	return client.Accounts.DeleteByID(accountToDelete.GetID())
}
