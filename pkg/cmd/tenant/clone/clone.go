package clone

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

const (
	FlagName         = "name"
	FlagDescription  = "description"
	FlagSourceTenant = "source-tenant"
)

type CloneFlags struct {
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	SourceTenant *flag.Flag[string]
	*machinescommon.WebFlags
}

func NewCloneFlags() *CloneFlags {
	return &CloneFlags{
		Name:         flag.New[string](FlagName, false),
		Description:  flag.New[string](FlagDescription, false),
		SourceTenant: flag.New[string](FlagSourceTenant, false),
	}
}

type CloneOptions struct {
	*CloneFlags
	*cmd.Dependencies
	GetTenantCallback     shared.GetTenantCallback
	GetAllTenantsCallback shared.GetAllTenantsCallback
}

func NewCloneOptions(flags *CloneFlags, dependencies *cmd.Dependencies) *CloneOptions {
	return &CloneOptions{
		CloneFlags:   flags,
		Dependencies: dependencies,
		GetTenantCallback: func(id string) (*tenants.Tenant, error) {
			return shared.GetTenant(*dependencies.Client, id)
		},
		GetAllTenantsCallback: func() ([]*tenants.Tenant, error) {
			return shared.GetAllTenants(*dependencies.Client)
		},
	}
}

func NewCmdClone(f factory.Factory) *cobra.Command {
	cloneFlags := NewCloneFlags()
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a tenant",
		Long:  "Clone a tenant in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant clone
			$ %[1]s tenant clone --name "Garys Cakes" --source-tenant "Bobs Wood Shop" 
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCloneOptions(cloneFlags, cmd.NewDependencies(f, c))

			return cloneRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&cloneFlags.Name.Value, cloneFlags.Name.Name, "n", "", "Name of the new tenant")
	flags.StringVarP(&cloneFlags.Description.Value, cloneFlags.Description.Name, "d", "", "Description of the new tenant")
	flags.StringVar(&cloneFlags.SourceTenant.Value, cloneFlags.SourceTenant.Name, "", "Name of the source tenant")
	return cmd
}

func cloneRun(opts *CloneOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	tenant, err := opts.GetTenantCallback(opts.SourceTenant.Value)
	if err != nil {
		return err
	}

	clonedTenant, err := opts.Client.Tenants.Clone(tenant, tenants.TenantCloneRequest{Name: opts.Name.Value, Description: opts.Description.Value})
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully cloned tenant '%s' to '%s'.\n", tenant.Name, clonedTenant.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.SourceTenant, opts.Name, opts.Description)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	link := output.Bluef("%s/app#/%s/tenants/%s/overview", opts.Host, opts.Space.GetID(), clonedTenant.GetID())
	fmt.Fprintf(opts.Out, "View this tenant on Octopus Deploy: %s\n", link)

	return nil
}

func PromptMissing(opts *CloneOptions) error {
	err := question.AskName(opts.Ask, "", "Tenant", &opts.Name.Value)
	if err != nil {
		return err
	}
	err = question.AskDescription(opts.Ask, "", "Tenant", &opts.Description.Value)
	if err != nil {
		return err
	}

	if opts.SourceTenant.Value == "" {
		tenant, err := selectors.Select(opts.Ask, "You have not specified a source Tenant to clone from. Please select one:", opts.GetAllTenantsCallback, func(tenant *tenants.Tenant) string {
			return tenant.Name
		})
		if err != nil {
			return err
		}

		opts.SourceTenant.Value = tenant.Name
	}

	return nil
}
