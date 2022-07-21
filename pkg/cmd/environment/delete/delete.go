package delete

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
		Short:   "Delete an environment in an instance of Octopus Deploy",
		Long:    "Delete an environment in an instance of Octopus Deploy.",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment delete
			$ %s environment rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("please specify the name or ID of the environment to delete")
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
			foundEnvironments, err := client.Environments.Get(environments.EnvironmentsQuery{
				// TODO we can't lookup by ID here because the server will AND it with the ItemName and produce no results
				PartialName: itemIDorName,
			})
			if err != nil {
				return err
			}
			// need exact match
			var environment *environments.Environment
			for _, item := range foundEnvironments.Items {
				if item.Name == itemIDorName {
					environment = item
					break
				}
			}
			if environment == nil {
				return fmt.Errorf("cannot find an environment with name or ID of '%s'", itemIDorName)
			}

			if !alreadyConfirmed { // TODO NO_PROMPT env var or whatever we do there
				err = question.AskForDeleteConfirmation("environment", environment.Name, environment.GetID())
				if err != nil {
					return err
				}
			}

			err = client.Environments.DeleteByID(environment.GetID())
			if err != nil { // e.g can't stop the task queue
				return err
			}

			cmd.Printf("Deleted Environment %s (%s).\n", environment.Name, environment.GetID())
			return err
		},
	}
	// TODO confirm might want to be a global flag?
	cmd.Flags().BoolP("confirm", "y", false, "Don't ask for confirmation before deleting the space.")

	return cmd
}
