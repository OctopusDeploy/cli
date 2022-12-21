package scopes

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"strings"
)

func ToScopeValues(variable *variables.Variable, vars variables.VariableSet) (*variables.VariableScopeValues, error) {
	scopeValues := &variables.VariableScopeValues{}

	var err error
	scopeValues.Environments, err = getSingleScope(variable.Scope.Environments, vars.ScopeValues.Environments)
	if err != nil {
		return nil, err
	}

	scopeValues.Channels, err = getSingleScope(variable.Scope.Channels, vars.ScopeValues.Channels)
	if err != nil {
		return nil, err
	}

	scopeValues.Actions, err = getSingleScope(variable.Scope.Actions, vars.ScopeValues.Actions)
	if err != nil {
		return nil, err
	}

	scopeValues.TenantTags, err = getSingleScope(variable.Scope.TenantTags, vars.ScopeValues.TenantTags)
	if err != nil {
		return nil, err
	}

	scopeValues.Roles, err = getSingleScope(variable.Scope.Roles, vars.ScopeValues.Roles)
	if err != nil {
		return nil, err
	}

	scopeValues.Machines, err = getSingleScope(variable.Scope.Machines, vars.ScopeValues.Machines)
	if err != nil {
		return nil, err
	}

	scopeValues.Processes, err = getSingleProcessScope(variable.Scope.ProcessOwners, vars.ScopeValues.Processes)
	if err != nil {
		return nil, err
	}

	return scopeValues, nil
}

func getSingleScope(scopes []string, lookupScopes []*resources.ReferenceDataItem) ([]*resources.ReferenceDataItem, error) {
	var referenceScopes []*resources.ReferenceDataItem
	for _, s := range scopes {
		scope, err := findSingleScope(s, lookupScopes)
		if err != nil {
			return nil, err
		}
		referenceScopes = append(referenceScopes, scope)
	}

	return referenceScopes, nil
}

func findSingleScope(scope string, scopes []*resources.ReferenceDataItem) (*resources.ReferenceDataItem, error) {
	for _, s := range scopes {
		if strings.EqualFold(scope, s.ID) {
			return s, nil
		}
	}

	return nil, fmt.Errorf("cannot find scope value for '%s'", scope)
}

func getSingleProcessScope(scopes []string, lookupScopes []*resources.ProcessReferenceDataItem) ([]*resources.ProcessReferenceDataItem, error) {
	var referenceScopes []*resources.ProcessReferenceDataItem
	for _, s := range scopes {
		scope, err := findSingleProcessScope(s, lookupScopes)
		if err != nil {
			return nil, err
		}
		referenceScopes = append(referenceScopes, scope)
	}

	return referenceScopes, nil
}

func findSingleProcessScope(scope string, scopes []*resources.ProcessReferenceDataItem) (*resources.ProcessReferenceDataItem, error) {
	for _, s := range scopes {
		if strings.EqualFold(scope, s.ID) {
			return s, nil
		}
	}

	return nil, fmt.Errorf("cannot find scope value for '%s'", scope)
}
