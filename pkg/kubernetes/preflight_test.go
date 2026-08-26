package kubernetes_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A loopback address can never resolve from inside a pod, so it is worth
// catching before scheduling anything. Local clusters are the common case here.
func TestStaticChecks_RejectsLoopbackAddresses(t *testing.T) {
	for _, address := range []string{
		"http://localhost:8065/",
		"https://127.0.0.1:8443",
		"grpc://localhost:8443",
		"0.0.0.0:8443",
	} {
		t.Run(address, func(t *testing.T) {
			checks := kubernetes.StaticChecks([]kubernetes.Target{{Name: "Octopus", Address: address}})
			require.Len(t, checks, 1)
			assert.Equal(t, kubernetes.CheckFailed, checks[0].Result)
			assert.Contains(t, checks[0].Remediation, "host.docker.internal")
		})
	}
}

func TestStaticChecks_AllowsRoutableAddresses(t *testing.T) {
	for _, address := range []string{
		"https://my.octopus.app",
		"grpc://my.octopus.app:8443",
		"https://octopus.internal:8080/",
		"host.docker.internal:8443",
		"10.0.0.5:8443",
	} {
		t.Run(address, func(t *testing.T) {
			checks := kubernetes.StaticChecks([]kubernetes.Target{{Name: "Octopus", Address: address}})
			assert.Empty(t, checks, "a routable address should raise nothing")
		})
	}
}

func TestStaticChecks_ReportsUnparseableAddresses(t *testing.T) {
	checks := kubernetes.StaticChecks([]kubernetes.Target{{Name: "Octopus", Address: "not a url"}})
	require.Len(t, checks, 1)
	assert.Equal(t, kubernetes.CheckFailed, checks[0].Result)
}
