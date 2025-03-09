package featuretoggle

import (
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/configuration"
)

// IsToggleEnabled retrieves an Octopus feature toggle by name. If an error occurs, it returns false.
func IsToggleEnabled(client *client.Client, toggleName string) (bool, error) {
	toggleRequest := &configuration.FeatureToggleConfigurationQuery{
		Name: toggleName,
	}

	returnToggleResponse, err := configuration.Get(client, toggleRequest)

	if err != nil {
		return false, err
	}

	if len(returnToggleResponse.FeatureToggles) == 0 {
		return false, err
	}

	var toggleValue = returnToggleResponse.FeatureToggles[0]

	// Verify name to avoid error in older versions of Octopus where Name param is not recognised
	if toggleValue.Name != toggleName {
		return false, err
	}

	return toggleValue.IsEnabled, err
}
