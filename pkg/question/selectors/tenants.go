package selectors

import (
	"errors"
	"fmt"
	"strings"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
)

// FindTenant looks a tenant up by either its ID or its name. An ID match wins over a name
// match, so it stays consistent with how projects, environments and channels resolve.
//
// Deliberately not Tenants.GetByIdentifier: its name fallback issues a single `partialName`
// (i.e. contains) query and only scans the first page of the result, so an exact name that
// sorts past that page is reported as not found. Names are on the deploy hot path and used
// to be resolved server side, so a miss here is a regression rather than an inconvenience.
func FindTenant(octopus *octopusApiClient.Client, tenantIdentifier string) (*tenants.Tenant, error) {
	if tenantIdentifier == "" {
		return nil, errors.New("cannot find a tenant without an ID or name")
	}

	tenant, err := octopus.Tenants.GetByID(tenantIdentifier)
	if err != nil {
		var apiError *core.APIError
		if errors.As(err, &apiError) && apiError.StatusCode != 404 {
			return nil, err
		}
		// a 404 (or an identifier that doesn't look like an ID at all) just means "try the name"
	} else if tenant != nil {
		return tenant, nil
	}

	resultPage, err := octopus.Tenants.Get(tenants.TenantsQuery{PartialName: tenantIdentifier})
	if err != nil {
		return nil, err
	}
	for resultPage != nil && len(resultPage.Items) > 0 {
		for _, t := range resultPage.Items { // the server has no exact-name search, so we emulate one
			if strings.EqualFold(t.Name, tenantIdentifier) {
				return t, nil
			}
		}
		resultPage, err = resultPage.GetNextPage(octopus.Tenants.GetClient())
		if err != nil {
			return nil, err
		} // if there are no more pages, GetNextPage returns nil, which breaks us out of the loop
	}

	return nil, fmt.Errorf("cannot find a tenant with the ID or name of '%s'", tenantIdentifier)
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
