package permissionscontroller_test

import (
	"testing"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/permissionscontroller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func clusterWith(resources []*metav1.APIResourceList) *octoK8s.Cluster {
	clientset := fake.NewSimpleClientset()
	clientset.Resources = resources
	return octoK8s.NewClusterForTesting(clientset, "test", "https://cluster")
}

func TestPresent(t *testing.T) {
	// The agent's own script pod templates share this API group, so the
	// controller has to be recognised by its resource rather than its group.
	agentOnly := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "agent.octopus.com/v1beta1",
		APIResources: []metav1.APIResource{{Name: "scriptpodtemplates", Kind: "ScriptPodTemplate"}},
	}})

	present, err := permissionscontroller.Present(agentOnly)
	require.NoError(t, err)
	assert.False(t, present)

	withController := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "agent.octopus.com/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "scriptpodtemplates", Kind: "ScriptPodTemplate"},
			{Name: "workloadserviceaccounts", Kind: "WorkloadServiceAccount"},
		},
	}})

	present, err = permissionscontroller.Present(withController)
	require.NoError(t, err)
	assert.True(t, present)
}

func TestCertManagerPresent(t *testing.T) {
	none := clusterWith(nil)
	present, err := permissionscontroller.CertManagerPresent(none)
	require.NoError(t, err)
	assert.False(t, present)

	installed := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "cert-manager.io/v1",
		APIResources: []metav1.APIResource{{Name: "certificates", Kind: "Certificate"}},
	}})
	present, err = permissionscontroller.CertManagerPresent(installed)
	require.NoError(t, err)
	assert.True(t, present)
}
