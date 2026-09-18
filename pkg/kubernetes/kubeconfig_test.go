package kubernetes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeKubeConfig(t *testing.T, contents string) *kubernetes.KubeConfig {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	kubeConfig, err := kubernetes.LoadKubeConfig(path)
	require.NoError(t, err)
	return kubeConfig
}

// An EKS kubeconfig authenticates by shelling out to the AWS CLI, which is
// where the cluster name and region come from.
func TestContexts_ReadsEKSDetailsFromTheExecPlugin(t *testing.T) {
	kubeConfig := writeKubeConfig(t, `
apiVersion: v1
kind: Config
current-context: eks
contexts:
  - name: eks
    context: {cluster: eks, user: eks}
clusters:
  - name: eks
    cluster: {server: "https://ABC123.gr7.ap-southeast-2.eks.amazonaws.com"}
users:
  - name: eks
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1beta1
        command: aws
        args: ["--region", "ap-southeast-2", "eks", "get-token", "--cluster-name", "my-cluster"]
        env:
          - name: AWS_PROFILE
            value: DeveloperAccess-123
`)

	contexts := kubeConfig.Contexts()
	require.Len(t, contexts, 1)

	eks := contexts[0].EKS
	require.NotNil(t, eks)
	assert.Equal(t, "my-cluster", eks.ClusterName)
	assert.Equal(t, "ap-southeast-2", eks.Region)
	assert.Equal(t, "DeveloperAccess-123", eks.Profile)
}

func TestContexts_NonEKSContextsHaveNoEKSDetails(t *testing.T) {
	kubeConfig := writeKubeConfig(t, `
apiVersion: v1
kind: Config
current-context: local
contexts:
  - name: local
    context: {cluster: local, user: local, namespace: argocd}
clusters:
  - name: local
    cluster: {server: "https://127.0.0.1:6443"}
users:
  - name: local
    user: {token: abc}
`)

	contexts := kubeConfig.Contexts()
	require.Len(t, contexts, 1)
	assert.Nil(t, contexts[0].EKS)
	assert.Equal(t, "argocd", contexts[0].Namespace)
	assert.True(t, contexts[0].IsCurrent)
}

func TestContexts_IgnoresNonAWSExecPlugins(t *testing.T) {
	kubeConfig := writeKubeConfig(t, `
apiVersion: v1
kind: Config
current-context: gke
contexts:
  - name: gke
    context: {cluster: gke, user: gke}
clusters:
  - name: gke
    cluster: {server: "https://34.40.169.235"}
users:
  - name: gke
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1beta1
        command: gke-gcloud-auth-plugin
`)

	require.Len(t, kubeConfig.Contexts(), 1)
	assert.Nil(t, kubeConfig.Contexts()[0].EKS)
}

func TestFindContext_ReportsUnknownNames(t *testing.T) {
	kubeConfig := writeKubeConfig(t, `
apiVersion: v1
kind: Config
current-context: local
contexts:
  - name: local
    context: {cluster: local, user: local}
clusters:
  - name: local
    cluster: {server: "https://127.0.0.1:6443"}
users:
  - name: local
    user: {token: abc}
`)

	_, err := kubeConfig.FindContext("nope")
	assert.ErrorContains(t, err, `no context named "nope"`)
}
