package delete

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var alreadyConfirmed bool
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a space in an instance of Octopus Deploy",
		Long:    "Delete a space in an instance of Octopus Deploy.",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space delete
			$ %s space rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deleteRun(f, cmd.OutOrStdout())
			}

			itemIDOrName := args[0]

			client, err := f.GetSystemClient()
			if err != nil {
				return err
			}
			// TODO go to the server and lookup the space id/name and how many projects it has
			itemToDelete, err := client.Spaces.GetByIDOrName(itemIDOrName)
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return fmt.Errorf("cannot find a space with name or ID of '%s'", itemIDOrName)
				}
				return err
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, "space", itemToDelete.Name, itemToDelete.ID, func() error {
					return delete(client, itemToDelete)
				})
			}

			return nil
		},
	}

	question.RegisterDeleteFlag(cmd, &alreadyConfirmed, "space")

	return cmd
}

func deleteRun(f factory.Factory, w io.Writer) error {
	client, err := f.GetSystemClient()
	if err != nil {
		return err
	}

	existingSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	itemToDelete, err := selectors.ByNameOrID(f.Ask, existingSpaces, "Select the space you wish to delete:")
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(f.Ask, "space", itemToDelete.Name, itemToDelete.ID, func() error {
		return delete(client, itemToDelete)
	})
}

func delete(client *client.Client, spaceToDelete *spaces.Space) error {
	if !spaceToDelete.TaskQueueStopped {
		spaceToDelete.TaskQueueStopped = true
		if _, err := client.Spaces.Update(spaceToDelete); err != nil {
			return err
		}
	}

	return client.Spaces.DeleteByID(spaceToDelete.ID)
}
