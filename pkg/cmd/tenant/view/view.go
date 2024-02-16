package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type ViewFlags struct {
	Web *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client   *client.Client
	Host     string
	out      io.Writer
	idOrName string
	flags    *ViewFlags
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a tenant",
		Long:  "View a tenant in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant view Tenants-1
			$ %[1]s tenant view 'Tenant'
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return fmt.Errorf("tenant identifier is required")
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				cmd.OutOrStdout(),
				args[0],
				viewFlags,
			}

			return viewRun(opts, cmd)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions, cmd *cobra.Command) error {
	tenant, err := opts.Client.Tenants.GetByIdentifier(opts.idOrName)
	if err != nil {
		return err
	}

	environmentMap, err := shared.GetEnvironmentMap(opts.Client, []*tenants.Tenant{tenant})
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
			Header: []string{"NAME", "DESCRIPTION", "ID", "TAGS"},
			Row: func(t *tenants.Tenant) []string {
				return []string{output.Bold(t.Name), t.Description, output.Dim(t.GetID()), output.FormatAsList(t.TenantTags)}
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

			link := fmt.Sprintf("%s/app#/%s/tenants/%s/overview", opts.Host, tenant.SpaceID, tenant.ID)
			s.WriteString(fmt.Sprintf("%s\n", link))

			// footer
			s.WriteString(fmt.Sprintf("View this tenant in Octopus Deploy: %s\n", output.Blue(link)))

			if opts.flags.Web.Value {
				browser.OpenURL(link)
			}

			return s.String()

			// fmt.Fprintf(opts.out, "%s %s\n", output.Bold(tenant.Name), output.Dimf("(%s)", tenant.ID))

			// if len(tenant.TenantTags) > 0 {
			// 	fmt.Fprintf(opts.out, "Tags: %s", output.FormatAsList(tenant.TenantTags))
			// 	fmt.Fprintf(opts.out, "\n")
			// }

			// if tenant.Description == "" {
			// 	fmt.Fprintln(opts.out, output.Dim(constants.NoDescription))
			// } else {
			// 	fmt.Fprintln(opts.out, output.Dim(tenant.Description))
			// }

			// link := fmt.Sprintf("%s/app#/%s/tenants/%s/overview", opts.Host, tenant.SpaceID, tenant.ID)

			// // footer
			// fmt.Fprintf(opts.out, "View this tenant in Octopus Deploy: %s\n", output.Blue(link))

			// if opts.flags.Web.Value {
			// 	browser.OpenURL(link)
			// }
			// return ""
		},
	})
}
