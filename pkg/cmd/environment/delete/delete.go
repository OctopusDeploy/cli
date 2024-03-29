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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var skipConfirmation bool
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete an environment",
		Long:    "Delete an environment in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s environment delete
			$ %[1]s environment rm
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return deleteRun(f, cmd)
			}

			itemIDOrName := args[0]

			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			// SDK doesn't have accounts.GetByIDOrName so we emulate it here
			foundEnvironments, err := client.Environments.Get(environments.EnvironmentsQuery{
				// TODO we can't lookup by ID here because the server will AND it with the ItemName and produce no results
				PartialName: itemIDOrName,
			})
			if err != nil {
				return err
			}
			// need exact match
			var itemToDelete *environments.Environment
			for _, item := range foundEnvironments.Items {
				if item.Name == itemIDOrName {
					itemToDelete = item
					break
				}
			}
			if itemToDelete == nil {
				return fmt.Errorf("cannot find an environment with name or ID of '%s'", itemIDOrName)
			}

			if !skipConfirmation { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, "environment", itemToDelete.Name, itemToDelete.GetID(), func() error {
					return delete(client, itemToDelete)
				})
			}

			return delete(client, itemToDelete)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &skipConfirmation, "environment")

	return cmd
}

func deleteRun(f factory.Factory, cmd *cobra.Command) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	existingItems, err := client.Environments.GetAll()
	if err != nil {
		return err
	}

	itemToDelete, err := selectors.ByName(f.Ask, existingItems, "Select the environment you wish to delete:")
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(f.Ask, "environment", itemToDelete.Name, itemToDelete.GetID(), func() error {
		return delete(client, itemToDelete)
	})
}

func delete(client *client.Client, itemToDelete *environments.Environment) error {
	return client.Environments.DeleteByID(itemToDelete.GetID())
}
