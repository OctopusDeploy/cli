package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

func EnvironmentsMultiSelect(ask question.Asker, client *client.Client, s factory.Spinner, message string) ([]string, error) {
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
	items, err := question.MultiSelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	})
	if err != nil {
		return nil, err
	}
	itemIds := make([]string, 0, len(items))
	for _, env := range items {
		itemIds = append(itemIds, env.GetID())
	}
	return itemIds, nil
}
