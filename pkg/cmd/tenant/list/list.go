package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenants",
		Long:  "List tenants in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant list
			$ %[1]s tenant ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f)
		},
	}

	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	allTenants, err := client.Tenants.GetAll()
	if err != nil {
		return err
	}

	environmentMap, err := shared.GetEnvironmentMap(client, allTenants)
	if err != nil {
		return err
	}

	projectMap, err := shared.GetProjectMap(client, allTenants)
	if err != nil {
		return err
	}

	return output.PrintArray(allTenants, cmd, output.Mappers[*tenants.Tenant]{
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
		Basic: func(t *tenants.Tenant) string {
			return t.Name
		},
	})
}
