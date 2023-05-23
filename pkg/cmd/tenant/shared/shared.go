package shared

import (
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/teams"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
)

type GetAllSpacesCallback func() ([]*spaces.Space, error)
type GetAllTeamsCallback func() ([]*teams.Team, error)
type GetAllUsersCallback func() ([]*users.User, error)
type GetAllTenantsCallback func() ([]*tenants.Tenant, error)
type GetTenantCallback func(identifier string) (*tenants.Tenant, error)
type GetAllProjectsCallback func() ([]*projects.Project, error)
type GetProjectCallback func(identifier string) (*projects.Project, error)
type GetProjectProgression func(project *projects.Project) (*projects.Progression, error)
type GetAllLibraryVariableSetsCallback func() ([]*variables.LibraryVariableSet, error)

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
