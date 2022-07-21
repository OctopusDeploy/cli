package delete

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func NewCmdDelete(f apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Args:    cobra.ExactArgs(1),
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
				return errors.New("Please specify the name or ID of the space to delete")
			}
			spaceIDorName := strings.TrimSpace(args[0])

			alreadyConfirmed, err := cmd.Flags().GetBool("confirm")
			if err != nil {
				return err
			}

			client, err := f.Get(false)
			if err != nil {
				return err
			}
			// TODO go to the server and lookup the space id/name and how many projects it has
			space, err := client.Spaces.GetByIDOrName(spaceIDorName)
			if err != nil { // could be services.itemNotFound if they typed it wrong.
				if err == services.ErrItemNotFound {
					return errors.New(fmt.Sprintf("Cannot find a space with name or ID of '%s'", spaceIDorName))
				}
				return err
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				confirmQuestion := &survey.Question{
					Name: "Confirm Delete",
					Prompt: &survey.Input{
						Message: fmt.Sprintf("Are you sure you want to delete the Space %s (%s). Enter yes or no:", space.Name, space.GetID()),
					},
				}

				var confirmStr string
				err = survey.Ask([]*survey.Question{confirmQuestion}, &confirmStr)
				if err != nil {
					return err
				}

				confirm, err := strconv.ParseBool(confirmStr)
				if err != nil {
					// not a parseable bool, try yes/no
					confirmStr = strings.ToLower(strings.TrimSpace(confirmStr))
					switch confirmStr {
					case "yes", "y", "ye":
						confirm = true
					default:
						confirm = false
					}
				}

				if !confirm {
					// user aborted
					return nil
				}

				// we need to stop the task queue on a space before we can delete it
				space.TaskQueueStopped = true
				space, err = client.Spaces.Update(space)
				if err != nil { // e.g can't stop the task queue
					return err
				}

				return client.Spaces.DeleteByID(space.GetID())
			}

			return nil
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the space.")

	return cmd
}
