package shared

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/question"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
)

const (
	// Unscoped labels a value that applies wherever no more specific value matches.
	Unscoped = "(unscoped)"
	// SensitiveValue stands in for a sensitive value, which the server never returns.
	SensitiveValue = "***"
)

// ResolveLibraryVariableSet finds the library variable set a command should operate
// on: looking it up when the caller named one, prompting for it in interactive mode
// when they didn't, and failing in automation mode where there is nobody to ask.
//
// The whole (space-scoped) list is fetched either way, so a caller-supplied name or
// ID is matched client-side rather than costing a second round trip.
func ResolveLibraryVariableSet(octopus *octopusApiClient.Client, ask question.Asker, promptEnabled bool, questionText string, idOrName string) (*variables.LibraryVariableSet, error) {
	if idOrName == "" && !promptEnabled {
		return nil, errors.New("library variable set must be specified")
	}

	allSets, err := sharedVariable.GetAllLibraryVariableSets(octopus)
	if err != nil {
		return nil, err
	}

	if idOrName == "" {
		if len(allSets) == 0 {
			return nil, errors.New("no library variable sets found")
		}
		return question.SelectMap(ask, questionText, allSets, func(s *variables.LibraryVariableSet) string { return s.Name })
	}

	for _, s := range allSets {
		if strings.EqualFold(s.GetID(), idOrName) || strings.EqualFold(s.Name, idOrName) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("cannot find library variable set '%s'", idOrName)
}

// ScopeAsJson mirrors the property names the Octopus API uses for a variable scope,
// but holds display names instead of IDs, falling back to the ID when a scope value
// isn't in the set's ScopeValues lookup.
type ScopeAsJson struct {
	Environments []string `json:"Environment,omitempty"`
	Roles        []string `json:"Role,omitempty"`
	Machines     []string `json:"Machine,omitempty"`
	TenantTags   []string `json:"TenantTag,omitempty"`
	Channels     []string `json:"Channel,omitempty"`
	Actions      []string `json:"Action,omitempty"`
	Processes    []string `json:"ProcessOwner,omitempty"`
}

// VariableValue is a single stored value of a variable, with its scope resolved.
type VariableValue struct {
	Id           string                           `json:"Id"`
	Value        string                           `json:"Value"`
	IsSensitive  bool                             `json:"IsSensitive"`
	Type         string                           `json:"Type,omitempty"`
	Description  string                           `json:"Description,omitempty"`
	IsScoped     bool                             `json:"IsScoped"`
	Scope        *ScopeAsJson                     `json:"Scope,omitempty"`
	ScopeSummary string                           `json:"ScopeSummary"`
	Prompt       *variables.VariablePromptOptions `json:"Prompt,omitempty"`
}

// DisplayValue is the value as it should be shown to a human.
func (v *VariableValue) DisplayValue() string {
	if v.IsSensitive {
		return SensitiveValue
	}
	return v.Value
}

// VariableGroup collects every value stored under one variable name. The API returns
// each scoped value as its own entry; grouping them is what makes a variable set with
// many scopes readable.
type VariableGroup struct {
	Name   string           `json:"Name"`
	Values []*VariableValue `json:"Values"`
}

// GroupVariables collapses a variable set's flat list into one group per variable
// name. Groups are ordered by name and, within a group, the unscoped value comes
// first because it is the fallback the scoped ones override.
func GroupVariables(variableSet *variables.VariableSet) []*VariableGroup {
	byName := map[string]*VariableGroup{}
	groups := []*VariableGroup{}

	for _, v := range variableSet.Variables {
		key := strings.ToLower(v.Name)
		group, ok := byName[key]
		if !ok {
			group = &VariableGroup{Name: v.Name}
			byName[key] = group
			groups = append(groups, group)
		}
		group.Values = append(group.Values, newVariableValue(v, variableSet.ScopeValues))
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	for _, group := range groups {
		values := group.Values
		sort.SliceStable(values, func(i, j int) bool {
			if values[i].IsScoped != values[j].IsScoped {
				return !values[i].IsScoped
			}
			return values[i].ScopeSummary < values[j].ScopeSummary
		})
	}

	return groups
}

func newVariableValue(v *variables.Variable, lookup *variables.VariableScopeValues) *VariableValue {
	value := &VariableValue{
		Id:           v.GetID(),
		Value:        v.Value,
		IsSensitive:  v.IsSensitive,
		Type:         v.Type,
		Description:  v.Description,
		IsScoped:     !v.Scope.IsEmpty(),
		ScopeSummary: Unscoped,
		Prompt:       v.Prompt,
	}
	if value.IsScoped {
		value.Scope = resolveScope(v.Scope, lookup)
		value.ScopeSummary = ScopeSummary(value.Scope)
	}
	return value
}

func resolveScope(scope variables.VariableScope, lookup *variables.VariableScopeValues) *ScopeAsJson {
	if lookup == nil {
		lookup = &variables.VariableScopeValues{}
	}
	return &ScopeAsJson{
		Environments: resolveNames(scope.Environments, lookup.Environments),
		Roles:        resolveNames(scope.Roles, lookup.Roles),
		Machines:     resolveNames(scope.Machines, lookup.Machines),
		// tenant tag scope values are canonical tag names already
		TenantTags: append([]string{}, scope.TenantTags...),
		Channels:   resolveNames(scope.Channels, lookup.Channels),
		Actions:    resolveNames(scope.Actions, lookup.Actions),
		Processes:  resolveProcessNames(scope.ProcessOwners, lookup.Processes),
	}
}

// ScopeSummary renders a scope as a single line, e.g. "Environment: Production; Role: web-server".
func ScopeSummary(scope *ScopeAsJson) string {
	if scope == nil {
		return Unscoped
	}

	parts := []string{}
	add := func(label string, values []string) {
		if len(values) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", label, strings.Join(values, ", ")))
		}
	}
	add("Environment", scope.Environments)
	add("Role", scope.Roles)
	add("Target", scope.Machines)
	add("Tenant tag", scope.TenantTags)
	add("Channel", scope.Channels)
	add("Step", scope.Actions)
	add("Process", scope.Processes)

	if len(parts) == 0 {
		return Unscoped
	}
	return strings.Join(parts, "; ")
}

func resolveNames(ids []string, refs []*resources.ReferenceDataItem) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, lookupName(id, refs))
	}
	return names
}

func resolveProcessNames(ids []string, refs []*resources.ProcessReferenceDataItem) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		name := id
		for _, r := range refs {
			if strings.EqualFold(r.ID, id) && r.Name != "" {
				name = r.Name
				break
			}
		}
		names = append(names, name)
	}
	return names
}

// lookupName falls back to the raw ID rather than erroring, so one unresolvable
// scope value can't stop the whole set from being displayed.
func lookupName(id string, refs []*resources.ReferenceDataItem) string {
	for _, r := range refs {
		if strings.EqualFold(r.ID, id) && r.Name != "" {
			return r.Name
		}
	}
	return id
}
