package selectors

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"strings"
)

func EnvironmentSelect(ask question.Asker, client *client.Client, message string) (*environments.Environment, error) {
	envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
	if err != nil {
		return nil, err
	}
	allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
	if err != nil {
		return nil, err
	}
	return question.SelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	})
}

func FindEnvironment(octopus *client.Client, environmentName string) (*environments.Environment, error) {
	resultPage, err := octopus.Environments.Get(environments.EnvironmentsQuery{PartialName: environmentName})
	if err != nil {
		return nil, err
	}
	// environmentsQuery has "Name" but it's just an alias in the server for PartialName; we need to filter client side
	for resultPage != nil && len(resultPage.Items) > 0 {
		for _, c := range resultPage.Items { // server doesn't support search by exact name so we must emulate it
			if strings.EqualFold(c.Name, environmentName) {
				return c, nil
			}
		}
		resultPage, err = resultPage.GetNextPage(octopus.Environments.GetClient())
		if err != nil {
			return nil, err
		} // if there are no more pages, then GetNextPage will return nil, which breaks us out of the loop
	}

	return nil, fmt.Errorf("no environment found with name of %s", environmentName)
}

func EnvironmentsMultiSelect(ask question.Asker, client *client.Client, message string, minItems int) ([]*environments.Environment, error) {
	envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
	if err != nil {
		return nil, err
	}
	allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
	if err != nil {
		return nil, err
	}
	return question.MultiSelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	}, minItems)
}
