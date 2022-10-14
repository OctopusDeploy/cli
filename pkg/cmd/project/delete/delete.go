package delete

import (
	"github.com/MakeNowJust/heredoc/v2"
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
	*question.DeleteFlags
}

func NewCmdList(f factory.Factory) *cobra.Command {
	deleteFlags := question.NewDeleteFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id> | <slug>}",
		Short:   "Delete projects in Octopus Deploy",
		Long:    "Delete projects in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Doc(`
			$ octopus project delete
			$ octopus project rm
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}

			opts := &DeleteOptions{
				Client:      client,
				Ask:         f.Ask,
				NoPrompt:    !f.IsPromptEnabled(),
				IdOrName:    args[0],
				DeleteFlags: deleteFlags,
			}

			return deleteRun(opts)
		},
	}

	question.RegisterDeleteFlag(cmd, &deleteFlags.Confirm.Value, "project")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	itemToDelete, err := opts.Client.Projects.GetByIdOrName(opts.IdOrName)
	if err != nil {
		return err
	}

	if opts.DeleteFlags.Confirm.Value {
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
