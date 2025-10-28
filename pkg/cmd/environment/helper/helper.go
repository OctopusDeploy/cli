package helper

import "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"

func GetByIDOrName(service *environments.EnvironmentService, idOrName string) (*environments.Environment, error) {
	// SDK doesn't have accounts.GetByIDOrName so we emulate it here
	foundEnvironments, err := service.Get(environments.EnvironmentsQuery{
		// TODO we can't lookup by ID here because the server will AND it with the ItemName and produce no results
		PartialName: idOrName,
	})
	if err != nil {
		return nil, err
	}
	// need exact match
	var matchedItem *environments.Environment
	for _, item := range foundEnvironments.Items {
		if item.Name == idOrName {
			matchedItem = item
			break
		}
	}

	return matchedItem, nil
}
