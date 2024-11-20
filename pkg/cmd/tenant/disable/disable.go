package disable

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

type DisableOptions struct {
	*cmd.Dependencies
	IdOrName string
}

func NewDisableOptions(args []string, dependencies *cmd.Dependencies) *DisableOptions {
	return &DisableOptions{
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func NewCmdDisable(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable {<name> | <id>}",
		Short: "Disable a tenant",
		Long:  "Disable a tenant in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s disable view Tenants-1
			$ %[1]s disable view 'Tenant'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, "")
			}

			opts := NewDisableOptions(args, cmd.NewDependencies(f, c))
			return disableRun(opts)
		},
	}

	return cmd
}

func disableRun(opts *DisableOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return fmt.Errorf("tenant identifier is required but was not provided")
	}

	tenantToUpdate, err := opts.Client.Tenants.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	tenantToUpdate.IsDisabled = true
	_, err = tenants.Update(opts.Client, tenantToUpdate)
	if err != nil {
		return err
	}

	return nil
}

func PromptMissing(opts *DisableOptions) error {
	if opts.IdOrName == "" {
		existingTenants, err := opts.Client.Tenants.GetAll()
		if err != nil {
			return err
		}
		selectedTenant, err := selectors.ByName(opts.Ask, existingTenants, "Select the tenant you wish to disable:")
		if err != nil {
			return err
		}
		opts.IdOrName = selectedTenant.GetID()
	}

	return nil
}
