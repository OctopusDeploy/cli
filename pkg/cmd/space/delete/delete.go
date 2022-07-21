package delete

import (
	"fmt"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f apiclient.ClientFactory) *cobra.Command {
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
				deleteRun(f, cmd.OutOrStdout())
				return nil
			}

			itemIDOrName := strings.TrimSpace(args[0])

			alreadyConfirmed, err := cmd.Flags().GetBool("confirm")
			if err != nil {
				return err
			}

			client, err := f.Get(false)
			if err != nil {
				return err
			}
			// TODO go to the server and lookup the space id/name and how many projects it has
			spaceToDelete, err := client.Spaces.GetByIDOrName(itemIDOrName)
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return fmt.Errorf("cannot find a space with name or ID of '%s'", itemIDOrName)
				}
				return err
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				err = question.AskForDeleteConfirmation(&question.SurveyAsker{}, "space", space.Name, space.GetID())
				if err != nil {
					return err
				}
			}

			// we need to stop the task queue on a space before we can delete it
			if !spaceToDelete.TaskQueueStopped {
				spaceToDelete.TaskQueueStopped = true
				if _, err := client.Spaces.Update(spaceToDelete); err != nil {
					return err
				}
			}

			if err := client.Spaces.DeleteByID(spaceToDelete.ID); err != nil {
				return err
			}

			cmd.Printf("%s The space, \"%s\" %s was deleted successfully.\n", output.Red("✔"), spaceToDelete.Name, output.Dimf("(%s)", spaceToDelete.ID))
			return nil
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the space.")

	return cmd
}

func deleteRun(f apiclient.ClientFactory, w io.Writer) error {
	client, err := f.Get(false)
	if err != nil {
		return err
	}

	existingSpaces, err := client.Spaces.GetAll()
	if err != nil {
		return err
	}

	spaceToDelete, err := selectSpace(existingSpaces, "Select the space you wish to delete:")
	if err != nil {
		return err
	}

	if err := question.AskForDeleteConfirmation("space", spaceToDelete.Name, spaceToDelete.ID); err != nil {
		return err
	}

	if !spaceToDelete.TaskQueueStopped {
		spaceToDelete.TaskQueueStopped = true
		if _, err := client.Spaces.Update(spaceToDelete); err != nil {
			return err
		}
	}

	if err := client.Spaces.DeleteByID(spaceToDelete.ID); err != nil {
		return err
	}

	fmt.Printf("%s The space, \"%s\" %s was deleted successfully.\n", output.Red("✔"), spaceToDelete.Name, output.Dimf("(%s)", spaceToDelete.ID))
	return nil
}

func selectSpace(existingSpaces []*spaces.Space, message string) (*spaces.Space, error) {
	var selectedSpace *spaces.Space
	if err := question.Select(message, existingSpaces, func(space *spaces.Space) string {
		for _, existingSpace := range existingSpaces {
			if space.ID == existingSpace.ID {
				return fmt.Sprintf("%s %s", space.Name, output.Dimf("(%s)", space.ID))
			}
		}
		return ""
	}, &selectedSpace); err != nil {
		return nil, err
	}

	return selectedSpace, nil
}
