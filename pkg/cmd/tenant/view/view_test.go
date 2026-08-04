package view

import (
	"errors"
	"testing"

	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/stretchr/testify/assert"
)

func TestResolveTenantIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		args     []string
		expected string
	}{
		{
			name:     "positional argument",
			args:     []string{"Bobs Wood Shop"},
			expected: "Bobs Wood Shop",
		},
		{
			name:     "tenant flag",
			tenant:   "Bobs Wood Shop",
			expected: "Bobs Wood Shop",
		},
		{
			name:     "tenant flag with an id",
			tenant:   "Tenants-123",
			expected: "Tenants-123",
		},
		{
			// consistent with the other commands accepting both forms
			name:     "flag wins when both are supplied",
			tenant:   "Bobs Wood Shop",
			args:     []string{"Sallys Tackle Truck"},
			expected: "Bobs Wood Shop",
		},
		{
			// nothing named, so the caller prompts for a tenant
			name:     "neither supplied",
			expected: "",
		},
		{
			name:     "no arguments does not panic",
			args:     []string{},
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, resolveTenantIdentifier(test.tenant, test.args))
		})
	}
}

func testTenants() []*tenants.Tenant {
	return []*tenants.Tenant{
		tenants.NewTenant("Tenant 1"),
		tenants.NewTenant("Tenant 2"),
	}
}

func TestSelectTenant_PromptsAndReturnsSelection(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("You have not specified a Tenant. Please select one:", "", []string{"Tenant 1", "Tenant 2"}, "Tenant 2"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	result, err := selectTenant(asker, func() ([]*tenants.Tenant, error) { return testTenants(), nil })

	checkRemainingPrompts()
	assert.NoError(t, err)
	// the selected tenant is returned whole, so viewRun need not look it up again
	assert.Equal(t, "Tenant 2", result.Name)
}

func TestSelectTenant_PropagatesLookupFailure(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	result, err := selectTenant(asker, func() ([]*tenants.Tenant, error) { return nil, errors.New("no tenants for you") })

	checkRemainingPrompts()
	assert.EqualError(t, err, "no tenants for you")
	assert.Nil(t, result)
}
