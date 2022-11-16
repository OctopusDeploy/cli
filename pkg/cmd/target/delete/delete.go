package delete

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	*cmd.Dependencies
	*shared.GetTargetsOptions
}

func NewDeleteOptions(dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		Dependencies:      dependencies,
		GetTargetsOptions: shared.NewGetTargetsOptionsForAllTargets(dependencies),
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	var skipConfirmation bool
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a deployment target in an instance of Octopus Deploy",
		Long:    "Delete a deployment target in an instance of Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target delete
			$ %s deployment-target rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			deps := cmd.NewDependencies(f, c)

			if util.Empty(args) {
				opts := NewDeleteOptions(deps)
				return deleteRun(opts)
			}

			idOrName := args[0]
			opts := NewDeleteOptions(deps)
			targets, err := opts.GetTargetsCallback()
			if err != nil {
				return err
			}

			var itemToDelete *machines.DeploymentTarget
			for _, item := range targets {
				if item.Name == idOrName || item.ID == idOrName {
					itemToDelete = item
					break
				}
			}

			if itemToDelete == nil {
				return fmt.Errorf("cannot find a deployment target with name or ID of '%s'", idOrName)
			}

			if !skipConfirmation { // TODO NO_PROMPT env var or whatever we do there
				return question.DeleteWithConfirmation(f.Ask, "deployment target", itemToDelete.Name, itemToDelete.GetID(), func() error {
					return delete(opts.Client, itemToDelete)
				})
			}

			return delete(opts.Client, itemToDelete)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &skipConfirmation, "deployment target")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	itemToDelete, err := selectors.Select(opts.Ask, "Select the deployment target you wish to delete:", opts.GetTargetsCallback, func(target *machines.DeploymentTarget) string { return target.Name })
	if err != nil {
		return err
	}

	return question.DeleteWithConfirmation(opts.Ask, "deployment target", itemToDelete.Name, itemToDelete.GetID(), func() error {
		return delete(opts.Client, itemToDelete)
	})
}

func delete(client *client.Client, itemToDelete *machines.DeploymentTarget) error {
	return client.Machines.DeleteByID(itemToDelete.GetID())
}
