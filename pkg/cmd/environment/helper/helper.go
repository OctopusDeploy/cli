package helper

import (
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

// GetByIDOrName returns the environment matching the given ID or exact name.
// The SDK has no environments.GetByIDOrName, so we emulate it here.
// Returns (nil, nil) when nothing matches; callers must handle that.
func GetByIDOrName(service *environments.EnvironmentService, idOrName string) (*environments.Environment, error) {
	// A 404 here just means the input wasn't an ID; anything else is a real error.
	environment, err := service.GetByID(idOrName)
	if err != nil {
		apiError, ok := err.(*core.APIError)
		if !ok || apiError.StatusCode != 404 {
			return nil, err
		}
	} else if environment != nil {
		return environment, nil
	}

	// The server only offers a partial name match, so we filter for the exact name.
	foundEnvironments, err := service.Get(environments.EnvironmentsQuery{
		PartialName: idOrName,
	})
	if err != nil {
		return nil, err
	}

	for _, item := range foundEnvironments.Items {
		if item.Name == idOrName {
			return item, nil
		}
	}

	return nil, nil
}
