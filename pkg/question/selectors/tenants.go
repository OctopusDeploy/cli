package selectors

import (
	"errors"
	"fmt"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
)

// FindTenant looks a tenant up by either its ID or its name.
func FindTenant(octopus *octopusApiClient.Client, tenantIdentifier string) (*tenants.Tenant, error) {
	tenant, err := octopus.Tenants.GetByIdentifier(tenantIdentifier)
	if err != nil {
		if errors.Is(err, services.ErrItemNotFound) {
			return nil, fmt.Errorf("cannot find a tenant with the ID or name of '%s'", tenantIdentifier)
		}
		return nil, err
	}
	return tenant, nil
}

// FindTenants looks tenants up by either their IDs or their names.
func FindTenants(octopus *octopusApiClient.Client, tenantIdentifiers []string) ([]*tenants.Tenant, error) {
	if len(tenantIdentifiers) == 0 {
		return nil, nil
	}
	result := make([]*tenants.Tenant, 0, len(tenantIdentifiers))
	for _, identifier := range tenantIdentifiers {
		tenant, err := FindTenant(octopus, identifier)
		if err != nil {
			return nil, err
		}
		result = append(result, tenant)
	}
	return result, nil
}
