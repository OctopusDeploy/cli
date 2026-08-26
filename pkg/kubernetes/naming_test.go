package kubernetes_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already a slug", "my-gateway", "my-gateway"},
		{"uppercase", "My Gateway", "my-gateway"},
		{"spaces collapse", "my   gateway", "my-gateway"},
		{"punctuation", "prod (eu-west) #1", "prod-eu-west-1"},
		{"leading and trailing junk", "  --Prod!!  ", "prod"},
		{"underscores", "my_gateway_01", "my-gateway-01"},
		{"unicode is dropped", "gateway-café", "gateway-caf"},
		{"digits only", "123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kubernetes.Slug(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSlug_NoUsableCharacters(t *testing.T) {
	for _, input := range []string{"", "   ", "---", "!!!", "☃"} {
		_, err := kubernetes.Slug(input)
		assert.Error(t, err, "expected %q to be rejected", input)
	}
}

func TestDerivedNamespace(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		input  string
		want   string
	}{
		{"gateway", kubernetes.ArgoCDGatewayNamespacePrefix, "verify", "octo-argo-gateway-verify"},
		{"agent", kubernetes.AgentNamespacePrefix, "colima", "octopus-agent-colima"},
		{"agent with spaces", kubernetes.AgentNamespacePrefix, "Nonproduction Agent", "octopus-agent-nonproduction-agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kubernetes.DerivedNamespace(tt.prefix, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDerivedNamespace_TruncatesToValidLabel(t *testing.T) {
	// 45 characters is all the gateway prefix leaves of the 63-character limit.
	long := "this-is-a-very-long-gateway-name-that-will-not-fit-in-a-namespace"
	got, err := kubernetes.DerivedNamespace(kubernetes.ArgoCDGatewayNamespacePrefix, long)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(got), 63)
	assert.Equal(t, "octo-argo-gateway-this-is-a-very-long-gateway-name-that-will", got)
	assertValidDNSLabel(t, got)
}

func TestDerivedNamespace_TruncationNeverLeavesTrailingHyphen(t *testing.T) {
	// Chosen so the naive cut lands exactly on a hyphen.
	got, err := kubernetes.DerivedNamespace(kubernetes.ArgoCDGatewayNamespacePrefix, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbb")
	require.NoError(t, err)

	assert.NotEmpty(t, got)
	assertValidDNSLabel(t, got)
}

func TestReleaseName_RespectsHelmLimit(t *testing.T) {
	got, err := kubernetes.ReleaseName("this-is-a-very-long-gateway-name-that-exceeds-helms-fifty-three-character-limit")
	require.NoError(t, err)

	assert.LessOrEqual(t, len(got), 53)
	assertValidDNSLabel(t, got)
}

func assertValidDNSLabel(t *testing.T, s string) {
	t.Helper()
	assert.Regexp(t, `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, s)
	assert.LessOrEqual(t, len(s), 63)
}
