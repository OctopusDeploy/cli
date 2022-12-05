package delete

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

const resourceDescription = "worker pool"

type DeleteOptions struct {
	*cmd.Dependencies
	*shared.GetWorkerPoolsOptions
}

func NewDeleteOptions(dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		Dependencies:          dependencies,
		GetWorkerPoolsOptions: shared.NewGetWorkerPoolsOptions(dependencies),
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var skipConfirmation bool

	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a worker pool",
		Long:    "Delete a worker pool in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s worker-pool delete
			$ %[1]s worker-pool rm
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			deps := cmd.NewDependencies(f, c)

			if util.Empty(args) {
				opts := NewDeleteOptions(deps)
				return deleteRun(opts)
			}

			idOrName := args[0]
			opts := NewDeleteOptions(deps)
			workerpool, err := opts.GetWorkerPoolCallback(idOrName)
			if err != nil {
				return err
			}

			if workerpool == nil {
				return fmt.Errorf("cannot find a worker with name or ID of '%s'", idOrName)
			}

			if !skipConfirmation { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, resourceDescription, workerpool.GetName(), workerpool.GetID(), func() error {
					return delete(opts.Client, workerpool.GetID())
				})
			}

			return delete(opts.Client, workerpool.GetID())
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &skipConfirmation, resourceDescription)

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	workerpool, err := selectors.Select(opts.Ask, "Select the worker pool you wish to delete:", opts.GetWorkerPoolsCallback, func(pool *workerpools.WorkerPoolListResult) string { return pool.Name })
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(opts.Ask, resourceDescription, workerpool.Name, workerpool.ID, func() error {
		return delete(opts.Client, workerpool.ID)
	})
}

func delete(client *client.Client, id string) error {
	return client.WorkerPools.DeleteByID(id)
}
