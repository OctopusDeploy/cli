package delete

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"

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
		Short:   "Delete a space in an instance of Octopus Deploy",
		Long:    "Delete a space in an instance of Octopus Deploy.",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space delete
			$ %s space rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("please specify the name or ID of the space to delete")
			}
			itemIDorName := strings.TrimSpace(args[0])

			alreadyConfirmed, err := cmd.Flags().GetBool("confirm")
			if err != nil {
				return err
			}

			client, err := f.Get(false)
			if err != nil {
				return err
			}
			// TODO go to the server and lookup the space id/name and how many projects it has
			space, err := client.Spaces.GetByIDOrName(itemIDorName)
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return fmt.Errorf("cannot find a space with name or ID of '%s'", itemIDorName)
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
			space.TaskQueueStopped = true
			space, err = client.Spaces.Update(space)
			if err != nil { // e.g can't stop the task queue
				return err
			}

			err = client.Spaces.DeleteByID(space.GetID())
			if err != nil { // e.g can't stop the task queue
				return err
			}

			cmd.Printf("Deleted Space %s (%s).\n", space.Name, space.GetID())
			return err
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the space.")

	return cmd
}
