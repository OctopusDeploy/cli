package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	Client   *client.Client
	Ask      question.Asker
	NoPrompt bool
	IdOrName string
	*question.ConfirmFlags
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete tenant in Octopus Deploy",
		Long:    "Delete tenant in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant delete
			$ %s tenant rm
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
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

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "tenant")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return fmt.Errorf("tenant identifier is required but was not provided")
	}

	itemToDelete, err := opts.Client.Tenants.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	if opts.Confirm.Value {
		return delete(opts.Client, itemToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "tenant", itemToDelete.Name, itemToDelete.ID, func() error {
			return delete(opts.Client, itemToDelete)
		})
	}
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.IdOrName == "" {
		existingTenants, err := opts.Client.Tenants.GetAll()
		if err != nil {
			return err
		}
		itemToDelete, err := selectors.Select(opts.Ask, "Select the tenant you wish to delete:", func() ([]*tenants.Tenant, error) {
			return existingTenants, nil
		}, func(item *tenants.Tenant) string {
			return item.Name
		})
		if err != nil {
			return err
		}
		opts.IdOrName = itemToDelete.GetID()
	}

	return nil
}

func delete(client *client.Client, tenant *tenants.Tenant) error {
	return client.Tenants.DeleteByID(tenant.GetID())
}
