package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helm reports only the values a release actually set, so every step of a path
// may be missing.
func TestValuesTree_TolerantOfMissingBranches(t *testing.T) {
	values := map[string]any{
		"agent": map[string]any{
			"name":   "Production",
			"worker": map[string]any{"enabled": true},
		},
	}

	assert.Equal(t, "Production", stringAt(values, "agent", "name"))
	assert.Equal(t, "", stringAt(values, "agent", "missing"))
	assert.Equal(t, "", stringAt(values, "nothing", "here", "at", "all"))
	assert.True(t, boolAt(values, false, "agent", "worker", "enabled"))
	assert.False(t, boolAt(values, false, "agent", "deploymentTarget", "enabled"))
	assert.True(t, boolAt(values, true, "agent", "deploymentTarget", "enabled"), "the fallback stands in for the chart's own default")

	// A value of the wrong type is no more useful than a missing one.
	assert.Equal(t, "", stringAt(values, "agent", "worker"))
}

func TestModeFrom(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]any
		expected Mode
	}{
		{"a worker", map[string]any{"agent": map[string]any{"worker": map[string]any{"enabled": true}}}, ModeWorker},
		{"a deployment target", map[string]any{"agent": map[string]any{"deploymentTarget": map[string]any{"enabled": true}}}, ModeDeploymentTarget},
		{"neither switched on", map[string]any{"agent": map[string]any{"name": "x"}}, ModeUnknown},
		{"values that could not be read", nil, ModeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, modeFrom(tt.values))
		})
	}
}

// The chart's default is a cluster-wide role for script pods, so a release that
// never mentioned it still has one.
func TestClusterRoleEnabled_DefaultsToTheChartsDefault(t *testing.T) {
	assert.True(t, clusterRoleEnabled(nil))
	assert.False(t, clusterRoleEnabled(map[string]any{
		"scriptPods": map[string]any{
			"serviceAccount": map[string]any{"clusterRole": map[string]any{"enabled": false}},
		},
	}))
}
