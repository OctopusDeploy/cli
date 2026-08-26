package helm

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helm resolves the namespace for any manifest that does not name one from the
// REST client getter, not from the install action. Leaving it unset puts a
// chart's resources in whatever namespace the kubeconfig context points at:
// the release is recorded in one namespace while its resources are created in
// another, and the install then waits for something that is not there.
func TestConfigFor_ScopesTheSettingsToTheTargetNamespace(t *testing.T) {
	runner, err := NewRunner("", "", io.Discard)
	require.NoError(t, err)

	_, err = runner.configFor("octo-argo-gateway-production")
	require.NoError(t, err)

	assert.Equal(t, "octo-argo-gateway-production", runner.settings.Namespace())
}

// Listing across namespaces passes no namespace, which must not be taken as a
// request to scope to the empty one.
func TestConfigFor_LeavesTheNamespaceAloneWhenNoneIsGiven(t *testing.T) {
	runner, err := NewRunner("", "", io.Discard)
	require.NoError(t, err)

	before := runner.settings.Namespace()
	_, err = runner.configFor("")
	require.NoError(t, err)

	assert.Equal(t, before, runner.settings.Namespace())
}
