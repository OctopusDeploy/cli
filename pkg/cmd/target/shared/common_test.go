package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDistinctRoles_EmptyList(t *testing.T) {
	result := shared.DistinctRoles([]string{})
	assert.Empty(t, result)
}

func TestDistinctRoles_DuplicateValues(t *testing.T) {
	result := shared.DistinctRoles([]string{"a", "b", "a"})
	assert.Equal(t, []string{"a", "b"}, result)
}
