package delete

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	Client   *client.Client
	Ask      question.Asker
	NoPrompt bool
	IdOrName string
	*question.ConfirmFlags
}

func NewCmdList(f factory.Factory) *cobra.Command {
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a project group",
		Long:    "Delete a project group in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s project-group delete
			$ %[1]s project-group rm
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}

			opts := &DeleteOptions{
				Client:       client,
				Ask:          f.Ask,
				NoPrompt:     !f.IsPromptEnabled(),
				IdOrName:     args[0],
				ConfirmFlags: confirmFlags,
			}

			return deleteRun(opts)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "project group")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	itemToDelete, err := opts.Client.ProjectGroups.GetByIDOrName(opts.IdOrName)
	if err != nil {
		return err
	}

	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, itemToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "project group", itemToDelete.Name, itemToDelete.ID, func() error {
			return delete(opts.Client, itemToDelete)
		})
	}
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.IdOrName == "" {
		existingProjects, err := opts.Client.ProjectGroups.GetAll()
		if err != nil {
			return err
		}
		itemToDelete, err := selectors.ByNameOrID(opts.Ask, existingProjects, "Select the project group you wish to delete:")
		if err != nil {
			return err
		}
		opts.IdOrName = itemToDelete.GetID()
	}

	return nil
}

func delete(client *client.Client, project *projectgroups.ProjectGroup) error {
	return client.ProjectGroups.DeleteByID(project.GetID())
}
