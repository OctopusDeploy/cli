package shared_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/libraryvariableset/shared"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
)

func newVariable(id string, name string, value string, scope variables.VariableScope) *variables.Variable {
	v := variables.NewVariable(name)
	v.ID = id
	v.Value = value
	v.Scope = scope
	return v
}

func newVariableSet(scopeValues *variables.VariableScopeValues, vars ...*variables.Variable) *variables.VariableSet {
	variableSet := fixtures.NewVariableSetForLibraryVariableSet("Spaces-1", "LibraryVariableSets-1")
	variableSet.ScopeValues = scopeValues
	variableSet.Variables = vars
	return variableSet
}

var scopeValues = &variables.VariableScopeValues{
	Environments: []*resources.ReferenceDataItem{
		{ID: "Environments-1", Name: "Production"},
		{ID: "Environments-2", Name: "Test"},
	},
	Roles: []*resources.ReferenceDataItem{
		{ID: "web-server", Name: "web-server"},
	},
}

func TestGroupVariables_CollapsesValuesSharingAName(t *testing.T) {
	variableSet := newVariableSet(scopeValues,
		newVariable("Variables-2", "Slack.Url", "https://prod", variables.VariableScope{Environments: []string{"Environments-1"}}),
		newVariable("Variables-1", "Slack.Url", "https://default", variables.VariableScope{}),
		newVariable("Variables-3", "Api.Key", "abc", variables.VariableScope{}),
	)

	groups := shared.GroupVariables(variableSet)

	assert.Equal(t, 2, len(groups))
	assert.Equal(t, "Api.Key", groups[0].Name)
	assert.Equal(t, 1, len(groups[0].Values))

	assert.Equal(t, "Slack.Url", groups[1].Name)
	assert.Equal(t, 2, len(groups[1].Values))
	// the unscoped value is the fallback the scoped ones override, so it comes first
	assert.Equal(t, "Variables-1", groups[1].Values[0].Id)
	assert.False(t, groups[1].Values[0].IsScoped)
	assert.Equal(t, shared.Unscoped, groups[1].Values[0].ScopeSummary)
	assert.Equal(t, "Variables-2", groups[1].Values[1].Id)
	assert.True(t, groups[1].Values[1].IsScoped)
	assert.Equal(t, "Environment: Production", groups[1].Values[1].ScopeSummary)
}

func TestGroupVariables_GroupsNamesCaseInsensitively(t *testing.T) {
	variableSet := newVariableSet(scopeValues,
		newVariable("Variables-1", "Slack.Url", "a", variables.VariableScope{}),
		newVariable("Variables-2", "slack.url", "b", variables.VariableScope{Environments: []string{"Environments-2"}}),
	)

	groups := shared.GroupVariables(variableSet)

	assert.Equal(t, 1, len(groups))
	assert.Equal(t, "Slack.Url", groups[0].Name)
	assert.Equal(t, 2, len(groups[0].Values))
}

func TestGroupVariables_ResolvesScopeIdsToNames(t *testing.T) {
	variableSet := newVariableSet(scopeValues,
		newVariable("Variables-1", "Db.Name", "orders", variables.VariableScope{
			Environments: []string{"Environments-1", "Environments-2"},
			Roles:        []string{"web-server"},
			TenantTags:   []string{"Regions/us-east"},
		}),
	)

	groups := shared.GroupVariables(variableSet)

	value := groups[0].Values[0]
	assert.Equal(t, []string{"Production", "Test"}, value.Scope.Environments)
	assert.Equal(t, []string{"web-server"}, value.Scope.Roles)
	assert.Equal(t, []string{"Regions/us-east"}, value.Scope.TenantTags)
	assert.Equal(t, "Environment: Production, Test; Role: web-server; Tenant tag: Regions/us-east", value.ScopeSummary)
}

func TestGroupVariables_FallsBackToTheIdWhenAScopeValueIsUnknown(t *testing.T) {
	variableSet := newVariableSet(scopeValues,
		newVariable("Variables-1", "Db.Name", "orders", variables.VariableScope{Environments: []string{"Environments-99"}}),
	)

	groups := shared.GroupVariables(variableSet)

	assert.Equal(t, "Environment: Environments-99", groups[0].Values[0].ScopeSummary)
}

func TestGroupVariables_ToleratesAMissingScopeValuesLookup(t *testing.T) {
	variableSet := newVariableSet(nil,
		newVariable("Variables-1", "Db.Name", "orders", variables.VariableScope{Environments: []string{"Environments-1"}}),
	)

	groups := shared.GroupVariables(variableSet)

	assert.Equal(t, "Environment: Environments-1", groups[0].Values[0].ScopeSummary)
}

func TestVariableValue_DisplayValueMasksSensitiveValues(t *testing.T) {
	sensitive := newVariable("Variables-1", "Db.Password", "", variables.VariableScope{})
	sensitive.IsSensitive = true
	variableSet := newVariableSet(scopeValues, sensitive)

	groups := shared.GroupVariables(variableSet)

	assert.Equal(t, shared.SensitiveValue, groups[0].Values[0].DisplayValue())
}
