package enable

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

type EnableOptions struct {
	*cmd.Dependencies
	IdOrName string
}

func NewEnableOptions(args []string, dependencies *cmd.Dependencies) *EnableOptions {
	return &EnableOptions{
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func NewCmdEnable(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enable",
		Short:   "Enable a tenant",
		Long:    "Enable a tenant in Octopus Deploy",
		Example: heredoc.Docf("$ %[1]s tenant enable", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, "")
			}

			opts := NewEnableOptions(args, cmd.NewDependencies(f, c))
			return enableRun(opts)
		},
	}

	return cmd
}

func enableRun(opts *EnableOptions) error {
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

	tenantToUpdate.IsDisabled = false
	_, err = tenants.Update(opts.Client, tenantToUpdate)
	if err != nil {
		return err
	}

	return nil
}

func PromptMissing(opts *EnableOptions) error {
	if opts.IdOrName == "" {
		existingTenants, err := opts.Client.Tenants.GetAll()
		if err != nil {
			return err
		}
		selectedTenant, err := selectors.ByName(opts.Ask, existingTenants, "Select the tenant you wish to enable:")
		if err != nil {
			return err
		}
		opts.IdOrName = selectedTenant.GetID()
	}

	return nil
}
