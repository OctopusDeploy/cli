package argocd_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AWS reports the capability endpoint as a bare hostname in some places and a
// URL in others; the chart always wants a grpc:// URL.
func TestNewManagedInstance_NormalisesTheEndpoint(t *testing.T) {
	want := "grpc://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com"

	for _, endpoint := range []string{
		"abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com",
		"https://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com",
		"https://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com/",
		"grpc://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com",
		"  abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com  ",
	} {
		t.Run(endpoint, func(t *testing.T) {
			assert.Equal(t, want, argocd.NewManagedInstance(endpoint).ServerGRPCURL)
		})
	}
}

// AWS serves managed Argo CD with a publicly trusted certificate through a load
// balancer that does not speak HTTP/2 - the opposite of a stock in-cluster
// install on both counts.
func TestNewManagedInstance_ConnectionSettings(t *testing.T) {
	instance := argocd.NewManagedInstance("abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com")

	assert.Equal(t, argocd.KindEKSManaged, instance.Kind)
	assert.True(t, instance.IsManaged())
	assert.True(t, instance.GRPCWeb, "AWS's load balancer does not support HTTP/2")
	assert.False(t, instance.SelfSignedTLS, "AWS uses a publicly trusted certificate")
	assert.False(t, instance.Plaintext)
}

func TestInstance_DisplayDistinguishesManaged(t *testing.T) {
	managed := argocd.NewManagedInstance("abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com")
	assert.Contains(t, managed.Display(), "AWS managed")

	inCluster := argocd.Instance{Kind: argocd.KindInCluster, ServiceName: "argocd-server", Namespace: "argocd", Version: "v3.4.2"}
	assert.Contains(t, inCluster.Display(), "namespace argocd")
	assert.False(t, inCluster.IsManaged())
}

// projectToken builds a token the way Argo CD does: the payload is base64url
// JSON, and the subject carries the project and role.
func projectToken(t *testing.T, subject string, expires int64) string {
	t.Helper()

	claims := map[string]any{"iss": "argocd", "sub": subject, "iat": 1783040347}
	if expires > 0 {
		claims["exp"] = expires
	}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + enc(payload) + ".c2lnbmF0dXJl"
}

// A person pasting a token should not also have to say which project it is for.
func TestParseProjectToken(t *testing.T) {
	claims, err := argocd.ParseProjectToken(projectToken(t, "proj:team-a:octopus", 0))
	require.NoError(t, err)

	assert.Equal(t, "team-a", claims.Project)
	assert.Equal(t, "octopus", claims.Role)
	assert.True(t, claims.Expires.IsZero())
	assert.False(t, claims.Expired())
}

func TestParseProjectToken_Expiry(t *testing.T) {
	expired, err := argocd.ParseProjectToken(projectToken(t, "proj:team-a:octopus", 1000000000))
	require.NoError(t, err)
	assert.True(t, expired.Expired())

	future, err := argocd.ParseProjectToken(projectToken(t, "proj:team-a:octopus", 4102444800))
	require.NoError(t, err)
	assert.False(t, future.Expired())
}

// An account token is the easy mistake to make, and AWS caps those at 12 hours
// so one would stop working within the day.
func TestParseProjectToken_RejectsAnAccountToken(t *testing.T) {
	_, err := argocd.ParseProjectToken(projectToken(t, "admin:apiKey", 0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an Argo CD project role token")
	assert.Contains(t, err.Error(), "admin:apiKey")
}

func TestParseProjectToken_RejectsRubbish(t *testing.T) {
	for _, input := range []string{"", "not-a-token", "a.b", "a.b.c.d", "abc.!!!notbase64!!!.xyz"} {
		_, err := argocd.ParseProjectToken(input)
		assert.Error(t, err, "expected %q to be rejected", input)
	}
}
