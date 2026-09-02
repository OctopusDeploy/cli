package install_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/accesstokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The monitor registers with the same short-lived token as the agent, so the
// two share one Secret and nothing extra reaches the Helm values.
func TestBuildValues_KubernetesMonitor(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.KubernetesMonitor.Value = true

	values, err := opts.BuildValues()
	require.NoError(t, err)

	monitor := values["kubernetesMonitor"].(map[string]any)
	assert.Equal(t, true, monitor["enabled"])
	assert.Equal(t, map[string]any{"serverGrpcUrl": "grpc://my.octopus.app:8443"}, monitor["monitor"])

	registration := monitor["registration"].(map[string]any)
	assert.Equal(t, octopusHost, registration["serverApiUrl"])
	assert.Equal(t, "Spaces-1", registration["spaceId"])
	assert.Equal(t, "Production", registration["machineName"])
	assert.Equal(t, "octopus-agent-registration-token", registration["serverAccessTokenSecretName"])
	assert.Equal(t, "bearer-token", registration["serverAccessTokenSecretKey"])
	assert.NotContains(t, registration, "serverAccessToken", "the access token must not reach the Helm values")
}

func TestBuildValues_KubernetesMonitorSharesTheInlineToken(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.KubernetesMonitor.Value = true
	opts.InlineSecrets.Value = true
	opts.Token = accesstokens.Token{Value: "eyJhbGciOiJIUzI1NiJ9.token"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	registration := values["kubernetesMonitor"].(map[string]any)["registration"].(map[string]any)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.token", registration["serverAccessToken"])
	assert.NotContains(t, registration, "serverAccessTokenSecretName")
}

func TestBuildValues_KubernetesMonitorTrustsTheSuppliedCertificate(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.KubernetesMonitor.Value = true
	opts.ServerCertificate.Value = "LS0tLS1CRUdJTg=="

	values, err := opts.BuildValues()
	require.NoError(t, err)

	registration := values["kubernetesMonitor"].(map[string]any)["registration"].(map[string]any)
	assert.Equal(t, "LS0tLS1CRUdJTg==", registration["serverCertificate"])
}

// The monitor watches the objects deployments create, which only a deployment
// target has, so a worker never installs one.
func TestBuildValues_WorkerNeverGetsTheMonitor(t *testing.T) {
	opts := completedWorkerOptions(t)
	opts.KubernetesMonitor.Value = true

	values, err := opts.BuildValues()
	require.NoError(t, err)

	assert.NotContains(t, values, "kubernetesMonitor")
}

func TestBuildValues_MonitorIsLeftOutUnlessAskedFor(t *testing.T) {
	values, err := completedTargetOptions(t).BuildValues()
	require.NoError(t, err)

	assert.NotContains(t, values, "kubernetesMonitor",
		"the subchart's own default keeps the monitor off, and setting anything would pin that")
}
