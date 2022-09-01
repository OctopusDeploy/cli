package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
