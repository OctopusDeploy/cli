package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

const resourceDescription = "worker"

type DeleteOptions struct {
	*cmd.Dependencies
	*shared.GetWorkersOptions
}

func NewDeleteOptions(dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		Dependencies:      dependencies,
		GetWorkersOptions: shared.NewGetWorkersOptions(dependencies, nil),
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var skipConfirmation bool

	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a worker",
		Long:    "Delete a worker in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s worker delete
			$ %[1]s worker rm
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			deps := cmd.NewDependencies(f, c)

			if util.Empty(args) {
				opts := NewDeleteOptions(deps)
				return deleteRun(opts)
			}

			idOrName := args[0]
			opts := NewDeleteOptions(deps)
			worker, err := opts.GetWorkerCallback(idOrName)
			if err != nil {
				return err
			}

			if worker == nil {
				return fmt.Errorf("cannot find a worker with name or ID of '%s'", idOrName)
			}

			if !skipConfirmation { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, resourceDescription, worker.Name, worker.GetID(), func() error {
					return delete(opts.Client, worker)
				})
			}

			return delete(opts.Client, worker)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &skipConfirmation, resourceDescription)

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	worker, err := selectors.Select(opts.Ask, "Select the worker you wish to delete:", opts.GetWorkersCallback, func(worker *machines.Worker) string { return worker.Name })
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(opts.Ask, resourceDescription, worker.Name, worker.GetID(), func() error {
		return delete(opts.Client, worker)
	})
}

func delete(client *client.Client, itemToDelete *machines.Worker) error {
	return client.Workers.DeleteByID(itemToDelete.GetID())
}
