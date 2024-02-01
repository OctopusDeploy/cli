package delete

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/spf13/cobra"
	"strings"
)

type DeleteOptions struct {
	Client       *client.Client
	Ask          question.Asker
	NoPrompt     bool
	UsernameOrId string
	*question.ConfirmFlags
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a user",
		Long:    "Delete a user in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s user delete some-user-name
			$ %[1]s user rm Users-123
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			octopus, err := f.GetSpacedClient(apiclient.NewRequester(c))
			if err != nil {
				return err
			}

			if len(args) == 0 {
				args = append(args, "")
			}

			opts := &DeleteOptions{
				Client:       octopus,
				Ask:          f.Ask,
				NoPrompt:     !f.IsPromptEnabled(),
				UsernameOrId: args[0],
				ConfirmFlags: confirmFlags,
			}

			return deleteRun(opts)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "user")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.UsernameOrId == "" {
		return fmt.Errorf("user identifier is required but was not provided")
	}

	// first we find by username
	// filter is not an exact match so it might find more than one row, but unlikely to find more than 2
	candidates, err := opts.Client.Users.Get(users.UsersQuery{Filter: opts.UsernameOrId, Take: 9999})
	if err != nil {
		return err
	}

	var itemToDelete *users.User = nil
	for _, candidate := range candidates.Items {
		if strings.EqualFold(opts.UsernameOrId, candidate.Username) {
			itemToDelete = candidate
			break
		}
	}
	if itemToDelete == nil { // couldn't match by username, try Id
		itemToDelete, err = opts.Client.Users.GetByID(opts.UsernameOrId)
		if err != nil {
			return err // can't find a user to delete. Give up
		}
	}

	if opts.Confirm.Value {
		return deleteUser(opts.Client, itemToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "user", itemToDelete.DisplayName, itemToDelete.ID, func() error {
			return deleteUser(opts.Client, itemToDelete)
		})
	}
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.UsernameOrId == "" {
		existingUsers, err := opts.Client.Users.GetAll()
		if err != nil {
			return err
		}
		itemToDelete, err := selectors.Select(opts.Ask, "Select the user you wish to delete:", func() ([]*users.User, error) {
			return existingUsers, nil
		}, func(item *users.User) string {
			return item.DisplayName
		})
		if err != nil {
			return err
		}
		opts.UsernameOrId = itemToDelete.GetID()
	}

	return nil
}

func deleteUser(client *client.Client, user *users.User) error {
	return client.Users.DeleteByID(user.GetID())
}
