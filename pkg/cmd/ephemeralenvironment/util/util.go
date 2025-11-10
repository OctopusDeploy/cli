package util

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
)

func GetByName(client *client.Client, name string, spaceID string) (*ephemeralenvironments.EphemeralEnvironment, error) {
	environments, err := ephemeralenvironments.GetByPartialName(client, spaceID, name)

	if err != nil {
		return nil, err
	}

	if environments.TotalResults == 0 {
		return nil, fmt.Errorf("no ephemeral environment found with the name '%s'", name)
	} else {
		var exactMatch *ephemeralenvironments.EphemeralEnvironment
		var toLowerName = strings.ToLower(name)

		for _, environment := range environments.Items {
			if strings.ToLower(environment.Name) == toLowerName {
				exactMatch = environment
				break
			}
		}

		if exactMatch != nil {
			return exactMatch, nil
		}

		return nil, fmt.Errorf("could not find an exact match of an ephemeral environment with the name '%s'. Please specify a more specific name", name)
	}
}
