package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
	"github.com/ztrue/tracerr"
)

type ProjectEnvironment struct {
	Project      output.IdAndName   `json:"Project"`
	Environments []output.IdAndName `json:"Environments"`
}

type TenantAsJson struct {
	*tenants.Tenant
	ProjectEnvironments []ProjectEnvironment
}

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
		return tracerr.Wrap(err)
	}

	allTenants, err := client.Tenants.GetAll()
	if err != nil {
		return tracerr.Wrap(err)
	}

	environmentMap, err := getEnvironmentMap(client, allTenants)
	if err != nil {
		return tracerr.Wrap(err)
	}

	projectMap, err := getProjectMap(client, allTenants)
	if err != nil {
		return tracerr.Wrap(err)
	}

	return output.PrintArray(allTenants, cmd, output.Mappers[*tenants.Tenant]{
		Json: func(t *tenants.Tenant) any {

			projectEnvironments := []ProjectEnvironment{}

			for p := range t.ProjectEnvironments {
				projectEntity := output.IdAndName{Id: p, Name: projectMap[p]}
				environments, err := resolveEntities(t.ProjectEnvironments[p], environmentMap)
				if err != nil {
					return tracerr.Wrap(err)
				}
				projectEnvironments = append(projectEnvironments, ProjectEnvironment{Project: projectEntity, Environments: environments})
			}

			t.Links = nil // ensure the links collection is not serialised
			return TenantAsJson{
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

func resolveEntities(keys []string, lookup map[string]string) ([]output.IdAndName, error) {
	var entities []output.IdAndName
	for _, k := range keys {
		entities = append(entities, output.IdAndName{Id: k, Name: lookup[k]})
	}

	return entities, nil
}

func getEnvironmentMap(client *client.Client, tenants []*tenants.Tenant) (map[string]string, error) {
	var environmentIds []string
	for _, t := range tenants {
		for p := range t.ProjectEnvironments {
			environmentIds = append(environmentIds, t.ProjectEnvironments[p]...)
		}
	}

	environmentIds = util.SliceDistinct(environmentIds)

	environmentMap := make(map[string]string)
	queryResult, err := client.Environments.Get(environments.EnvironmentsQuery{IDs: environmentIds})
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	allEnvs, err := queryResult.GetAllPages(client.Environments.GetClient())
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	for _, e := range allEnvs {
		environmentMap[e.GetID()] = e.GetName()
	}
	return environmentMap, nil
}

func getProjectMap(client *client.Client, tenants []*tenants.Tenant) (map[string]string, error) {
	var projectIds []string
	for _, t := range tenants {
		for p := range t.ProjectEnvironments {
			projectIds = append(projectIds, p)
		}
	}
	projectIds = util.SliceDistinct(projectIds)

	projectMap := make(map[string]string)
	queryResult, err := client.Projects.Get(projects.ProjectsQuery{IDs: projectIds})
	allProjects, err := queryResult.GetAllPages(client.Projects.GetClient())
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	for _, e := range allProjects {
		projectMap[e.GetID()] = e.GetName()
	}
	return projectMap, nil
}
