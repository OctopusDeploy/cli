package list

import (
	"testing"

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
