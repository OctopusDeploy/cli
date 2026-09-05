package helm_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
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

	assert.Equal(t, "Production", helm.StringAt(values, "agent", "name"))
	assert.Equal(t, "", helm.StringAt(values, "agent", "missing"))
	assert.Equal(t, "", helm.StringAt(values, "nothing", "here", "at", "all"))
	assert.True(t, helm.BoolAt(values, false, "agent", "worker", "enabled"))
	assert.False(t, helm.BoolAt(values, false, "agent", "deploymentTarget", "enabled"))
	assert.True(t, helm.BoolAt(values, true, "agent", "deploymentTarget", "enabled"), "the fallback stands in for the chart's own default")

	// A value of the wrong type is no more useful than a missing one.
	assert.Equal(t, "", helm.StringAt(values, "agent", "worker"))
}
