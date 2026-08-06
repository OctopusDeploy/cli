package view

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagTenant = "tenant"
	FlagWeb    = "web"
)

type ViewFlags struct {
	Tenant *flag.Flag[string]
	Web    *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Tenant: flag.New[string](FlagTenant, false),
		Web:    flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client *client.Client
	Host   string
	out    io.Writer
	tenant *tenants.Tenant
	flags  *ViewFlags
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.MaximumNArgs(1),
		Use:   "view [<name> | <id>]",
		Short: "View a tenant",
		Long:  "View a tenant in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s tenant view Tenants-1
			%[1]s tenant view 'Tenant'
			%[1]s tenant view --tenant 'Tenant'
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			tenant, err := resolveTenant(f, client, resolveTenantIdentifier(viewFlags.Tenant.Value, args))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				cmd.OutOrStdout(),
				tenant,
				viewFlags,
			}

			return viewRun(opts, cmd)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&viewFlags.Tenant.Value, viewFlags.Tenant.Name, "t", "", "The tenant")
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

// resolveTenantIdentifier prefers --tenant and falls back to the positional
// argument, matching how the other commands accepting both forms behave.
//
// An empty result means no tenant was named; the caller prompts for one.
func resolveTenantIdentifier(tenant string, args []string) string {
	if tenant != "" {
		return tenant
	}

	if len(args) > 0 {
		return args[0]
	}

	return ""
}

// resolveTenant looks up the named tenant, or prompts for one when the command
// line did not name any. With prompting disabled there is nothing to select
// from, so the identifier is required.
func resolveTenant(f factory.Factory, octopus *client.Client, idOrName string) (*tenants.Tenant, error) {
	if idOrName != "" {
		return octopus.Tenants.GetByIdentifier(idOrName)
	}

	if !f.IsPromptEnabled() {
		return nil, fmt.Errorf("must supply tenant identifier")
	}

	return promptMissing(f.Ask, func() ([]*tenants.Tenant, error) { return shared.GetAllTenants(octopus) })
}

// promptMissing prompts for a tenant. The getter is a parameter so the prompt can
// be driven from tests, as connect and update do.
func promptMissing(ask question.Asker, getAllTenants shared.GetAllTenantsCallback) (*tenants.Tenant, error) {
	return selectors.Select(
		ask,
		"You have not specified a Tenant. Please select one:",
		getAllTenants,
		func(tenant *tenants.Tenant) string { return tenant.Name })
}

func viewRun(opts *ViewOptions, cmd *cobra.Command) error {
	tenant := opts.tenant

	environmentMap, err := shared.GetEnvironmentMapForTenants(opts.Client, []*tenants.Tenant{tenant})
	if err != nil {
		return err
	}

	projectMap, err := shared.GetProjectMap(opts.Client, []*tenants.Tenant{tenant})
	if err != nil {
		return err
	}

	return output.PrintResource(tenant, cmd, output.Mappers[*tenants.Tenant]{
		Json: func(t *tenants.Tenant) any {

			projectEnvironments := []shared.ProjectEnvironment{}

			for p := range t.ProjectEnvironments {
				projectEntity := output.IdAndName{Id: p, Name: projectMap[p]}
				environments, err := shared.ResolveEntities(t.ProjectEnvironments[p], environmentMap)
				if err != nil {
					return err
				}
				projectEnvironments = append(projectEnvironments, shared.ProjectEnvironment{Project: projectEntity, Environments: environments})
			}

			t.Links = nil // ensure the links collection is not serialised
			return shared.TenantAsJson{
				Tenant:              t,
				ProjectEnvironments: projectEnvironments,
			}
		},
		Table: output.TableDefinition[*tenants.Tenant]{
			Header: []string{"NAME", "DESCRIPTION", "ID", "IS DISABLED", "TAGS"},
			Row: func(t *tenants.Tenant) []string {
				return []string{output.Bold(t.Name), t.Description, output.Dim(t.GetID()), strconv.FormatBool(t.IsDisabled), output.FormatAsList(t.TenantTags)}
			},
		},
		Basic: func(item *tenants.Tenant) string {
			var s strings.Builder

			s.WriteString(fmt.Sprintf("%s %s\n", output.Bold(tenant.Name), output.Dimf("(%s)", tenant.ID)))

			if len(tenant.TenantTags) > 0 {
				s.WriteString(fmt.Sprintf("Tags: %s\n", output.FormatAsList(tenant.TenantTags)))
			}

			if tenant.Description == "" {
				s.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
			} else {
				s.WriteString(fmt.Sprintln(output.Dim(tenant.Description)))
			}

			if tenant.IsDisabled {
				s.WriteString(fmt.Sprintln("Tenant is disabled"))
			} else {
				s.WriteString(fmt.Sprintln("Tenant is enabled"))
			}

			link := util.GenerateWebURL(opts.Host, tenant.SpaceID, fmt.Sprintf("tenants/%s/overview", tenant.ID))
			// footer
			s.WriteString(fmt.Sprintf("View this tenant in Octopus Deploy: %s\n", output.Blue(link)))

			if opts.flags.Web.Value {
				browser.OpenURL(link)
			}

			return s.String()
		},
	})
}
