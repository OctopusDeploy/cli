package delete

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
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
			accountIDorName := strings.TrimSpace(args[0])

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
				IDs:         []string{accountIDorName}, // seems to be OR, which is what we want here
				PartialName: accountIDorName,
			})
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return fmt.Errorf("cannot find an account with name or ID of '%s'", accountIDorName)
				}
				return err
			}

			// need exact match
			var account accounts.IAccount
			for _, act := range foundAccounts.Items {
				if act.GetName() == accountIDorName {
					account = act
					break
				}
			}
			if account == nil {
				return fmt.Errorf("cannot find an account with name or ID of '%s'", accountIDorName)
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				confirmQuestion := &survey.Question{
					Name: "Confirm Delete",
					Prompt: &survey.Input{
						Message: fmt.Sprintf(`You are about to delete the account, "%s" %s. This action cannot be reversed. To confirm, type the account name:`, account.GetName(), output.Dimf("(%s)", account.GetID())),
					},
				}

				var accountName string
				if err = survey.Ask([]*survey.Question{confirmQuestion}, &accountName); err != nil {
					return err
				}
				if accountName != strings.TrimSpace(account.GetName()) {
					// user aborted
					return errors.New("Canceled")
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
