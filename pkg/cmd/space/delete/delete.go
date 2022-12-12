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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

// TODO this command has no tests and doesn't follow the same patterns as our newer commands
func NewCmdDelete(f factory.Factory) *cobra.Command {
	var alreadyConfirmed bool
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a space",
		Long:    "Delete a space in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s space delete
			$ %[1]s space rm
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deleteRun(f, cmd)
			}

			itemIDOrName := args[0]

			octopus, err := f.GetSystemClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}
			// TODO slugs
			itemToDelete, err := octopus.Spaces.GetByIDOrName(itemIDOrName)
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return fmt.Errorf("cannot find a space with name or ID of '%s'", itemIDOrName)
				}
				return err
			}

			if alreadyConfirmed {
				return stopQueueAndDelete(octopus, itemToDelete)
			} else {
				// TODO handle NO_PROMPT or CI mode. Should we require everyone to explicitly --confirm in CI mode or should we just go delete it?
				// feels like being explicit would be better
				return question.DeleteWithConfirmation(f.Ask, "space", itemToDelete.Name, itemToDelete.ID, func() error {
					return stopQueueAndDelete(octopus, itemToDelete)
				})
			}
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &alreadyConfirmed, "space")

	return cmd
}

func deleteRun(f factory.Factory, cmd *cobra.Command) error {
	octopus, err := f.GetSystemClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	existingSpaces, err := octopus.Spaces.GetAll()
	if err != nil {
		return err
	}

	itemToDelete, err := selectors.ByNameOrID(f.Ask, existingSpaces, "Select the space you wish to delete:")
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(f.Ask, "space", itemToDelete.Name, itemToDelete.ID, func() error {
		return stopQueueAndDelete(octopus, itemToDelete)
	})
}

func stopQueueAndDelete(client *client.Client, spaceToDelete *spaces.Space) error {
	if !spaceToDelete.TaskQueueStopped {
		spaceToDelete.TaskQueueStopped = true
		if _, err := client.Spaces.Update(spaceToDelete); err != nil {
			return err
		}
	}

	return client.Spaces.DeleteByID(spaceToDelete.ID)
}
