package shared

import (
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
)

type ProjectEnvironment struct {
	Project      output.IdAndName   `json:"Project"`
	Environments []output.IdAndName `json:"Environments"`
}

type TenantAsJson struct {
	*tenants.Tenant
	ProjectEnvironments []ProjectEnvironment
}

type GetAllSpacesCallback func() ([]*spaces.Space, error)
type GetAllTeamsCallback func() ([]*teams.Team, error)
type GetAllUsersCallback func() ([]*users.User, error)
type GetAllTenantsCallback func() ([]*tenants.Tenant, error)
type GetTenantCallback func(identifier string) (*tenants.Tenant, error)
type GetAllProjectsCallback func() ([]*projects.Project, error)
type GetProjectCallback func(identifier string) (*projects.Project, error)
type GetProjectProgression func(project *projects.Project) (*projects.Progression, error)
type GetAllLibraryVariableSetsCallback func() ([]*variables.LibraryVariableSet, error)

func ResolveEntities(keys []string, lookup map[string]string) ([]output.IdAndName, error) {
	var entities []output.IdAndName
	for _, k := range keys {
		entities = append(entities, output.IdAndName{Id: k, Name: lookup[k]})
	}

	return entities, nil
}

func GetEnvironmentMap(client *client.Client, tenants []*tenants.Tenant) (map[string]string, error) {
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
		return nil, err
	}
	allEnvs, err := queryResult.GetAllPages(client.Environments.GetClient())
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	for _, e := range allEnvs {
		environmentMap[e.GetID()] = e.GetName()
	}
	return environmentMap, nil
}

func GetProjectMap(client *client.Client, tenants []*tenants.Tenant) (map[string]string, error) {
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
		return nil, err
	}
	for _, e := range allProjects {
		projectMap[e.GetID()] = e.GetName()
	}
	return projectMap, nil
}

func GetAllTeams(client client.Client) ([]*teams.Team, error) {
	res, err := client.Teams.Get(teams.TeamsQuery{IncludeSystem: true})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func GetAllUsers(client client.Client) ([]*users.User, error) {
	res, err := client.Users.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetAllSpaces(client client.Client) ([]*spaces.Space, error) {
	res, err := client.Spaces.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetAllTenants(client *client.Client) ([]*tenants.Tenant, error) {
	res, err := client.Tenants.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetTenant(client *client.Client, identifier string) (*tenants.Tenant, error) {
	res, err := client.Tenants.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetAllProjects(client *client.Client) ([]*projects.Project, error) {
	res, err := client.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetProject(client *client.Client, identifier string) (*projects.Project, error) {
	res, err := client.Projects.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetAllLibraryVariableSets(client *client.Client) ([]*variables.LibraryVariableSet, error) {
	res, err := client.LibraryVariableSets.GetAll()
	if err != nil {
		return nil, err
	}

	return util.SliceFilter(res, func(item *variables.LibraryVariableSet) bool { return item.ContentType == "Variables" }), nil
}
