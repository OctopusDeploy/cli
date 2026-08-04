package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveTenantIdentifier(t *testing.T) {
	tests := []struct {
		name          string
		tenant        string
		args          []string
		expected      string
		expectedError string
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
			name:          "both supplied is ambiguous",
			tenant:        "Bobs Wood Shop",
			args:          []string{"Sallys Tackle Truck"},
			expectedError: "tenant specified as both an argument and with --tenant, please use only one",
		},
		{
			name:          "neither supplied",
			expectedError: "must supply tenant identifier",
		},
		{
			name:          "empty positional argument",
			args:          []string{""},
			expectedError: "must supply tenant identifier",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := resolveTenantIdentifier(test.tenant, test.args)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.Empty(t, result)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}
