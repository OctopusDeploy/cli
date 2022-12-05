package delete

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
		Use:     "delete {<name> | <id> | <slug>}",
		Short:   "Delete a project",
		Long:    "Delete a project in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s project delete
			$ %[1]s project rm
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			if len(args) == 0 {
				args = append(args, "")
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

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "project")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return fmt.Errorf("project identifier is required but was not provided")
	}

	itemToDelete, err := opts.Client.Projects.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, itemToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "project", itemToDelete.Name, itemToDelete.ID, func() error {
			return delete(opts.Client, itemToDelete)
		})
	}
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.IdOrName == "" {
		existingProjects, err := opts.Client.Projects.GetAll()
		if err != nil {
			return err
		}
		itemToDelete, err := selectors.ByNameOrID(opts.Ask, existingProjects, "Select the project you wish to delete:")
		if err != nil {
			return err
		}
		opts.IdOrName = itemToDelete.GetID()
	}

	return nil
}

func delete(client *client.Client, project *projects.Project) error {
	return client.Projects.DeleteByID(project.GetID())
}
