package shared

import (
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
)

type GetAllTenantsCallback func() ([]*tenants.Tenant, error)
type GetTenantCallback func(identifier string) (*tenants.Tenant, error)
type GetAllProjectsCallback func() ([]*projects.Project, error)
type GetProjectCallback func(identifier string) (*projects.Project, error)
type GetProjectProgression func(project *projects.Project) (*projects.Progression, error)

func GetAllTenants(client client.Client) ([]*tenants.Tenant, error) {
	res, err := client.Tenants.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetTenant(client client.Client, identifier string) (*tenants.Tenant, error) {
	res, err := client.Tenants.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetAllProjects(client client.Client) ([]*projects.Project, error) {
	res, err := client.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetProject(client client.Client, identifier string) (*projects.Project, error) {
	res, err := client.Projects.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return res, nil
}
