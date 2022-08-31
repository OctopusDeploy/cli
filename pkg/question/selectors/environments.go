package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

func EnvironmentSelect(ask question.Asker, client *client.Client, s factory.Spinner, message string) (*environments.Environment, error) {
	s.Start()
	envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
	if err != nil {
		s.Stop()
		return nil, err
	}
	allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.Stop()
	return question.SelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	})
}

func EnvironmentsMultiSelect(ask question.Asker, client *client.Client, s factory.Spinner, message string, minItems int) ([]*environments.Environment, error) {
	s.Start()
	envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
	if err != nil {
		s.Stop()
		return nil, err
	}
	allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.Stop()
	return question.MultiSelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	}, minItems)
}
