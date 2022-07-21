package delete

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Args:    usage.ExactArgs(1),
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
				return errors.New("please specify the name or ID of the account to delete")
			}
			itemIDorName := strings.TrimSpace(args[0])

			alreadyConfirmed, err := cmd.Flags().GetBool("confirm")
			if err != nil {
				return err
			}

			client, err := f.Get(true) // space-scoped
			if err != nil {
				return err
			}

			// SDK doesn't have accounts.GetByIDOrName so we emulate it here
			foundAccounts, err := client.Accounts.Get(accounts.AccountsQuery{
				// TODO we can't lookup by ID yet because the server doesn't support it
				PartialName: itemIDorName,
			})
			if err != nil {
				return err
			}
			// need exact match
			var account accounts.IAccount
			for _, act := range foundAccounts.Items {
				if act.GetName() == itemIDorName {
					account = act
					break
				}
			}
			if account == nil {
				return fmt.Errorf("cannot find an account with name or ID of '%s'", itemIDorName)
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				err := question.AskForDeleteConfirmation(&question.SurveyAsker{}, "account", account.GetName(), account.GetID())
				if err != nil {
					return err
				}
			}

			err = client.Accounts.DeleteByID(account.GetID())
			if err != nil { // e.g can't stop the task queue
				return err
			}

			cmd.Printf("Deleted Account %s (%s).\n", account.GetName(), account.GetID())
			return err
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the account.")

	return cmd
}
