package agent_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func clusterWith(resources []*metav1.APIResourceList, objects ...runtime.Object) *octoK8s.Cluster {
	clientset := fake.NewSimpleClientset(objects...)
	clientset.Resources = resources
	return octoK8s.NewClusterForTesting(clientset, "test", "https://cluster")
}

func TestPermissionsControllerPresent(t *testing.T) {
	// The agent's own script pod templates share this API group, so the
	// controller has to be recognised by its resource rather than its group.
	agentOnly := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "agent.octopus.com/v1beta1",
		APIResources: []metav1.APIResource{{Name: "scriptpodtemplates", Kind: "ScriptPodTemplate"}},
	}})

	present, err := agent.PermissionsControllerPresent(agentOnly)
	require.NoError(t, err)
	assert.False(t, present)

	withController := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "agent.octopus.com/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "scriptpodtemplates", Kind: "ScriptPodTemplate"},
			{Name: "workloadserviceaccounts", Kind: "WorkloadServiceAccount"},
		},
	}})

	present, err = agent.PermissionsControllerPresent(withController)
	require.NoError(t, err)
	assert.True(t, present)
}

func TestCertManagerPresent(t *testing.T) {
	none := clusterWith(nil)
	present, err := agent.CertManagerPresent(none)
	require.NoError(t, err)
	assert.False(t, present)

	installed := clusterWith([]*metav1.APIResourceList{{
		GroupVersion: "cert-manager.io/v1",
		APIResources: []metav1.APIResource{{Name: "certificates", Kind: "Certificate"}},
	}})
	present, err = agent.CertManagerPresent(installed)
	require.NoError(t, err)
	assert.True(t, present)
}

func TestStorageClasses_DefaultFirst(t *testing.T) {
	cluster := clusterWith(nil,
		&storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: "filestore"},
			Provisioner: "filestore.csi.storage.gke.io",
		},
		&storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "standard",
				Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"},
			},
			Provisioner: "kubernetes.io/gce-pd",
		})

	classes, err := cluster.StorageClasses(context.Background())
	require.NoError(t, err)
	require.Len(t, classes, 2)

	assert.Equal(t, "standard", classes[0].Name)
	assert.True(t, classes[0].IsDefault)
	assert.Equal(t, "standard (cluster default, kubernetes.io/gce-pd)", classes[0].Display())
	assert.Equal(t, "filestore (filestore.csi.storage.gke.io)", classes[1].Display())
}

func TestArchitectureSupport(t *testing.T) {
	tests := []struct {
		name        string
		present     []string
		unsupported []string
		runnable    bool
	}{
		{"a cluster of arm64 nodes", []string{"arm64"}, nil, true},
		{"a mixed cluster still has somewhere to run", []string{"amd64", "s390x"}, []string{"s390x"}, true},
		{"nowhere to run", []string{"s390x"}, []string{"s390x"}, false},
		{"nodes that could not be listed are assumed to be fine", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.unsupported, agent.UnsupportedArchitectures(tt.present))
			assert.Equal(t, tt.runnable, agent.RunnableArchitecture(tt.present))
		})
	}
}

// Retrying cannot change a cluster's node architectures, so the error is typed
// for the installer to recognise.
func TestErrUnsupportedNodes(t *testing.T) {
	err := error(agent.ErrUnsupportedNodes{Architectures: []string{"s390x", "ppc64le"}})

	assert.Contains(t, err.Error(), "s390x, ppc64le")
	assert.Contains(t, err.Error(), "linux/amd64 and linux/arm64")

	var typed agent.ErrUnsupportedNodes
	assert.True(t, errors.As(fmt.Errorf("wrapped: %w", err), &typed))
	assert.Equal(t, []string{"s390x", "ppc64le"}, typed.Architectures)
}

// The Kubernetes API does not report which access modes a storage class
// supports, so the provisioner is the only signal there is.
func TestStorageClass_SupportsReadWriteMany(t *testing.T) {
	tests := map[string]bool{
		"filestore.csi.storage.gke.io": true,
		"efs.csi.aws.com":              true,
		"file.csi.azure.com":           true,
		"nfs.csi.k8s.io":               true,
		"cephfs.csi.ceph.com":          true,
		"kubernetes.io/gce-pd":         false,
		"ebs.csi.aws.com":              false,
		"disk.csi.azure.com":           false,
		"rancher.io/local-path":        false,
		"":                             false,
		// An unrecognised provisioner costs node affinity; assuming the other
		// way leaves a volume that never binds.
		"example.com/something-new": false,
	}

	for provisioner, expected := range tests {
		t.Run(provisioner, func(t *testing.T) {
			assert.Equal(t, expected, octoK8s.StorageClass{Provisioner: provisioner}.SupportsReadWriteMany())
		})
	}
}

// The chart checks the value is a list and renders it with toYaml, neither of
// which a typed struct survives.
func TestPolicyRuleValues_ArePlainMapsWithoutEmptyFields(t *testing.T) {
	values := octoK8s.PolicyRuleValues([]rbacv1.PolicyRule{
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list"}},
		{NonResourceURLs: []string{"/healthz"}, Verbs: []string{"get"}},
		{},
	})

	assert.Equal(t, []any{
		map[string]any{"apiGroups": []string{"apps"}, "resources": []string{"deployments"}, "verbs": []string{"get", "list"}},
		map[string]any{"nonResourceURLs": []string{"/healthz"}, "verbs": []string{"get"}},
	}, values, "a rule that grants nothing is dropped, and so is every empty field")
}

func TestRole_Display(t *testing.T) {
	unrestricted := octoK8s.Role{Name: "cluster-admin", Rules: []rbacv1.PolicyRule{
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
	}}
	assert.True(t, unrestricted.GrantsEverything())
	assert.Equal(t, "cluster-admin (cluster role, full access to the cluster)", unrestricted.Display(),
		"copying this restricts nothing, which is worth saying before somebody picks it")

	scoped := octoK8s.Role{Name: "deployer", Rules: []rbacv1.PolicyRule{
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get"}},
	}}
	assert.False(t, scoped.GrantsEverything())
	assert.Equal(t, "deployer (cluster role, 1 rule)", scoped.Display())

	namespaced := octoK8s.Role{Name: "reader", Namespace: "monitoring"}
	assert.Equal(t, "monitoring/reader", namespaced.Reference(), "a namespaced role is named the way a flag names it")
	assert.Equal(t, "monitoring/reader (role, 0 rules)", namespaced.Display())
}

// Kubernetes ships around seventy roles of its own, and the kube- namespaces
// hold the control plane's. None of them is what anybody is looking for here.
func TestRoles_LeavesOutTheOnesKubernetesShips(t *testing.T) {
	cluster := clusterWith(nil,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "system:discovery"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "deployer"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "admin"}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "reader", Namespace: "monitoring"}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "signer", Namespace: "kube-system"}})

	roles, err := cluster.Roles(context.Background())
	require.NoError(t, err)

	references := make([]string, 0, len(roles))
	for _, role := range roles {
		references = append(references, role.Reference())
	}
	assert.Equal(t, []string{"admin", "deployer", "monitoring/reader"}, references,
		"cluster roles come first, because they are the more likely answer")
}

// RBAC is additive, so a workload given several roles ends up with the union.
func TestMergePolicyRules_UnionWithoutDuplicates(t *testing.T) {
	pods := rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}
	deployments := rbacv1.PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get"}}

	merged := octoK8s.MergePolicyRules([]octoK8s.Role{
		{Name: "a", Rules: []rbacv1.PolicyRule{pods, deployments}},
		{Name: "b", Rules: []rbacv1.PolicyRule{pods}},
	})

	assert.Equal(t, []any{
		map[string]any{"apiGroups": []string{""}, "resources": []string{"pods"}, "verbs": []string{"get"}},
		map[string]any{"apiGroups": []string{"apps"}, "resources": []string{"deployments"}, "verbs": []string{"get"}},
	}, merged)
}
