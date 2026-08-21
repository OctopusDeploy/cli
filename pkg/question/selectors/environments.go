package selectors

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

type GetAllEnvironmentsCallback func() ([]*environments.Environment, error)

func GetAllEnvironments(client *client.Client) ([]*environments.Environment, error) {
	envResources, err := client.Environments.Get(environments.EnvironmentsQuery{})
	if err != nil {
		return nil, err
	}
	allEnvs, err := envResources.GetAllPages(client.Environments.GetClient())
	if err != nil {
		return nil, err
	}

	return allEnvs, nil
}

func EnvironmentSelect(ask question.Asker, getAllEnvironmentsCallback GetAllEnvironmentsCallback, message string) (*environments.Environment, error) {
	allEnvs, err := getAllEnvironmentsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	})
}

// FindEnvironment looks an environment up by either its ID or its name.
func FindEnvironment(octopus *client.Client, environmentIdentifier string) (*environments.Environment, error) {
	found, err := FindEnvironments(octopus, []string{environmentIdentifier})
	if err != nil {
		return nil, err
	}
	return found[0], nil
}

// FindEnvironments looks environments up by either their IDs or their names. An ID match
// wins over a name match, so it stays consistent with how projects and tenants resolve.
func FindEnvironments(octopus *client.Client, environmentIdentifiers []string) ([]*environments.Environment, error) {
	if len(environmentIdentifiers) == 0 {
		return nil, nil
	}
	// there's no "bulk lookup" API, so we either need to do a foreach loop to find each environment individually, or load the entire server's worth of environments
	// it's probably going to be cheaper to just list out all the environments and match them client side, so we'll do that for simplicity's sake
	allEnvs, err := octopus.Environments.GetAll()
	if err != nil {
		return nil, err
	}

	idLookup := make(map[string]*environments.Environment, len(allEnvs))
	nameLookup := make(map[string]*environments.Environment, len(allEnvs))
	for _, env := range allEnvs {
		idLookup[strings.ToLower(env.GetID())] = env
		nameLookup[strings.ToLower(env.GetName())] = env
	}

	result := make([]*environments.Environment, 0, len(environmentIdentifiers))
	for _, identifier := range environmentIdentifiers {
		key := strings.ToLower(identifier)
		env, found := idLookup[key]
		if !found {
			env, found = nameLookup[key]
		}
		if !found {
			return nil, fmt.Errorf("cannot find an environment with the ID or name of '%s'", identifier)
		}
		result = append(result, env)
	}
	return result, nil
}

func EnvironmentsMultiSelect(ask question.Asker, getAllEnvironmentsCallback GetAllEnvironmentsCallback, message string, required bool) ([]*environments.Environment, error) {
	allEnvs, err := getAllEnvironmentsCallback()
	if err != nil {
		return nil, err
	}
	return question.MultiSelectMap(ask, message, allEnvs, func(item *environments.Environment) string {
		return item.Name
	}, required)
}
