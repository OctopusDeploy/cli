package delete

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete an account in an instance of Octopus Deploy",
		Long:    "Delete an account in an instance of Octopus Deploy.",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account delete
			$ %s account rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deleteRun(f, cmd.OutOrStdout())
			}

			itemIDOrName := args[0]

			skipConfirmation, err := cmd.Flags().GetBool("confirm")
			if err != nil {
				return err
			}

			client, err := f.Client(true) // space-scoped
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
				return question.AskForDeleteConfirmation(f.Ask, "space", itemToDelete.GetName(), itemToDelete.GetID(), func() error {
					return delete(client, itemToDelete)
				})
			}

			return delete(client, itemToDelete)
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the account.")

	return cmd
}

func deleteRun(f factory.Factory, w io.Writer) error {
	client, err := f.Client(true)
	if err != nil {
		return err
	}

	existingAccounts, err := client.Accounts.GetAll()
	if err != nil {
		return err
	}

	accountToDelete, err := selectors.Account(f.Ask, existingAccounts, "Select the account you wish to delete:")
	if err != nil {
		return err
	}

	return question.AskForDeleteConfirmation(f.Ask, "account", accountToDelete.GetName(), accountToDelete.GetID(), func() error {
		return delete(client, accountToDelete)
	})
}

func delete(client *client.Client, accountToDelete accounts.IAccount) error {
	return client.Accounts.DeleteByID(accountToDelete.GetID())
}
