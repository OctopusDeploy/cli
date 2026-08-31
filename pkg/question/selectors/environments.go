package selectors

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
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
	lookup := newIdentifierLookup(allEnvs,
		func(env *environments.Environment) string { return env.GetID() },
		func(env *environments.Environment) string { return env.GetName() })

	result := make([]*environments.Environment, 0, len(environmentIdentifiers))
	for _, identifier := range environmentIdentifiers {
		env, found := lookup.find(identifier)
		if !found {
			return nil, fmt.Errorf("cannot find an environment with the ID or name of '%s'", identifier)
		}
		result = append(result, env)
	}
	return result, nil
}

// ResolveEnvironmentNames maps environment names or IDs onto canonical environment names, because
// the executions API only matches environments by name.
//
// Ephemeral environments aren't part of the regular environment list, so that list is consulted -
// once, lazily - for any identifier the regular list doesn't have. Resolving one identifier at a
// time means a list mixing the two kinds still reports the identifier that actually went missing.
func ResolveEnvironmentNames(octopus *client.Client, space *spaces.Space, environmentIdentifiers []string) ([]string, error) {
	if len(environmentIdentifiers) == 0 {
		return nil, nil
	}
	allEnvs, err := octopus.Environments.GetAll()
	if err != nil {
		return nil, err
	}
	regular := newIdentifierLookup(allEnvs,
		func(env *environments.Environment) string { return env.GetID() },
		func(env *environments.Environment) string { return env.GetName() })

	var ephemeral *identifierLookup[*ephemeralenvironments.EphemeralEnvironment]

	names := make([]string, 0, len(environmentIdentifiers))
	for _, identifier := range environmentIdentifiers {
		if env, found := regular.find(identifier); found {
			names = append(names, env.GetName())
			continue
		}

		if ephemeral == nil {
			if space == nil {
				return nil, fmt.Errorf("cannot find an environment with the ID or name of '%s'", identifier)
			}
			allEphemeral, ephemeralErr := ephemeralenvironments.GetAll(octopus, space.ID)
			if ephemeralErr != nil {
				// ephemeral environments are the rarer case, and the endpoint doesn't exist on
				// every server; either way the identifier is genuinely not a regular environment
				return nil, fmt.Errorf("cannot find an environment with the ID or name of '%s'", identifier)
			}
			lookup := newIdentifierLookup(allEphemeral.Items,
				func(env *ephemeralenvironments.EphemeralEnvironment) string { return env.ID },
				func(env *ephemeralenvironments.EphemeralEnvironment) string { return env.Name })
			ephemeral = &lookup
		}

		if env, found := ephemeral.find(identifier); found {
			names = append(names, env.Name)
			continue
		}
		return nil, fmt.Errorf("cannot find an environment with the ID or name of '%s'", identifier)
	}
	return names, nil
}

// identifierLookup indexes items by both ID and name so an identifier can be matched against
// either, with an ID match winning when an item's name collides with another item's ID.
type identifierLookup[T any] struct {
	byID   map[string]T
	byName map[string]T
}

func newIdentifierLookup[T any](items []T, id func(T) string, name func(T) string) identifierLookup[T] {
	lookup := identifierLookup[T]{
		byID:   make(map[string]T, len(items)),
		byName: make(map[string]T, len(items)),
	}
	for _, item := range items {
		lookup.byID[strings.ToLower(id(item))] = item
		lookup.byName[strings.ToLower(name(item))] = item
	}
	return lookup
}

func (l identifierLookup[T]) find(identifier string) (T, bool) {
	key := strings.ToLower(identifier)
	if item, found := l.byID[key]; found {
		return item, true
	}
	item, found := l.byName[key]
	return item, found
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
