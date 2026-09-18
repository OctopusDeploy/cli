package argocd_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"

	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cm(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "argocd"},
		Data:       data,
	}
}

// inClusterInstance is an Argo CD installed straight into the cluster, where
// the ConfigMaps are the source of truth.
func inClusterInstance() argocd.Instance {
	return argocd.Instance{Kind: argocd.KindInCluster, Namespace: "argocd"}
}

func readOnlySpec() argocd.AccountSpec {
	return argocd.AccountSpec{Name: "octopus", AllowSync: false}
}

func TestInspectAccount_NothingConfigured(t *testing.T) {
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{}))

	status, err := argocd.InspectAccount(context.Background(), c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)

	assert.False(t, status.HasAPIKeyCapability)
	assert.Len(t, status.MissingPolicies, 3)
	assert.False(t, status.IsComplete())
}

func TestInspectAccount_FullyConfigured(t *testing.T) {
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		cm(argocd.RBACConfigMapName, map[string]string{"policy.csv": `
p, octopus, applications, get, *, allow
p, octopus, clusters, get, *, allow
p, octopus, logs, get, */*, allow
`}),
	)

	status, err := argocd.InspectAccount(context.Background(), c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	assert.True(t, status.IsComplete())
}

func TestInspectAccount_SyncNeedsAnExtraPolicy(t *testing.T) {
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		cm(argocd.RBACConfigMapName, map[string]string{"policy.csv": `
p, octopus, applications, get, *, allow
p, octopus, clusters, get, *, allow
p, octopus, logs, get, */*, allow
`}),
	)

	status, err := argocd.InspectAccount(context.Background(), c, inClusterInstance(),
		argocd.AccountSpec{Name: "octopus", AllowSync: true})
	require.NoError(t, err)

	require.Len(t, status.MissingPolicies, 1)
	assert.Contains(t, status.MissingPolicies[0], "sync")
}

// Whitespace in policy.csv is a formatting choice, not a difference in meaning,
// so it must not cause a duplicate rule to be added.
func TestInspectAccount_PolicyMatchingIgnoresWhitespace(t *testing.T) {
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		cm(argocd.RBACConfigMapName, map[string]string{"policy.csv": "p,octopus,applications,get,*,allow\n" +
			"p,   octopus,  clusters,  get,  *,  allow\n" +
			"p, octopus, logs, get, */*, allow"}),
	)

	status, err := argocd.InspectAccount(context.Background(), c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	assert.Empty(t, status.MissingPolicies)
}

func TestInspectAccount_DisabledAccountIsNotComplete(t *testing.T) {
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{
		"accounts.octopus":         "apiKey",
		"accounts.octopus.enabled": "false",
	}))

	status, err := argocd.InspectAccount(context.Background(), c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)

	assert.True(t, status.HasAPIKeyCapability)
	assert.True(t, status.Disabled)
	assert.False(t, status.IsComplete())
}

func TestConfigureAccount_LeavesOtherAccountsAndPoliciesAlone(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{
			"accounts.alice": "login, apiKey",
			"url":            "https://argo.example.com",
		}),
		cm(argocd.RBACConfigMapName, map[string]string{
			"policy.csv":     "p, alice, applications, *, *, allow\n",
			"policy.default": "role:readonly",
		}),
	)

	status, err := argocd.InspectAccount(ctx, c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	require.NoError(t, argocd.ConfigureAccount(ctx, c, inClusterInstance(), status))

	config, found, err := c.GetConfigMap(ctx, "argocd", argocd.ConfigMapName)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "apiKey", config.Data["accounts.octopus"])
	assert.Equal(t, "true", config.Data["accounts.octopus.enabled"])
	assert.Equal(t, "login, apiKey", config.Data["accounts.alice"], "another account must not be disturbed")
	assert.Equal(t, "https://argo.example.com", config.Data["url"], "unrelated keys must not be disturbed")

	rbac, found, err := c.GetConfigMap(ctx, "argocd", argocd.RBACConfigMapName)
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, rbac.Data["policy.csv"], "p, alice, applications, *, *, allow", "an existing rule must survive")
	assert.Contains(t, rbac.Data["policy.csv"], "p, octopus, clusters, get, *, allow")
	assert.Equal(t, "role:readonly", rbac.Data["policy.default"])

	// Running it a second time must not duplicate anything.
	status, err = argocd.InspectAccount(ctx, c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	assert.True(t, status.IsComplete())
}

func TestConfigureAccount_CreatesTheRBACConfigMapWhenMissing(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{}))

	status, err := argocd.InspectAccount(ctx, c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	require.NoError(t, argocd.ConfigureAccount(ctx, c, inClusterInstance(), status))

	rbac, found, err := c.GetConfigMap(ctx, "argocd", argocd.RBACConfigMapName)
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, rbac.Data["policy.csv"], "p, octopus, applications, get, *, allow")
}

func TestConfigureAccount_PreservesExistingCapabilities(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "login"}))

	status, err := argocd.InspectAccount(ctx, c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)
	require.NoError(t, argocd.ConfigureAccount(ctx, c, inClusterInstance(), status))

	config, _, err := c.GetConfigMap(ctx, "argocd", argocd.ConfigMapName)
	require.NoError(t, err)
	assert.Equal(t, "login, apiKey", config.Data["accounts.octopus"])
}

func TestAccountPatchPlan_ShowsOnlyWhatIsMissing(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}))

	status, err := argocd.InspectAccount(ctx, c, inClusterInstance(), readOnlySpec())
	require.NoError(t, err)

	plan := argocd.AccountPatchPlan("argocd", status)
	assert.NotContains(t, plan, argocd.ConfigMapName, "the account already exists, so argocd-cm needs no change")
	assert.Contains(t, plan, argocd.RBACConfigMapName)
	assert.Contains(t, plan, "p, octopus, clusters, get, *, allow")
}

func argoCDResource(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1beta1",
		"kind":       "ArgoCD",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec":       map[string]any{},
	}}
}

func operatorInstance() argocd.Instance {
	return argocd.Instance{
		Kind:      argocd.KindInCluster,
		Namespace: "openshift-gitops",
		Operator: &argocd.OperatorInstance{
			Name:     "openshift-gitops",
			Resource: schema.GroupVersionResource{Group: "argoproj.io", Version: "v1beta1", Resource: "argocds"},
		},
	}
}

func operatorCluster(objects ...runtime.Object) *octoK8s.Cluster {
	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1beta1", Resource: "argocds"}
	listKinds := map[schema.GroupVersionResource]string{gvr: "ArgoCDList"}

	c := clusterWith(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: argocd.ConfigMapName, Namespace: "openshift-gitops"}},
	)
	return c.WithDynamic(dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...))
}

// An operator generates argocd-cm and argocd-rbac-cm from its own resource, so
// anything written straight to them is reverted on the next reconcile.
func TestConfigureAccount_OperatorManagedWritesToTheArgoCDResource(t *testing.T) {
	ctx := context.Background()
	instance := operatorInstance()
	c := operatorCluster(argoCDResource("openshift-gitops", "openshift-gitops"))

	status, err := argocd.InspectAccount(ctx, c, instance, readOnlySpec())
	require.NoError(t, err)
	require.NoError(t, argocd.ConfigureAccount(ctx, c, instance, status))

	argoCD, err := c.Dynamic.Resource(instance.Operator.Resource).Namespace(instance.Namespace).
		Get(ctx, instance.Operator.Name, metav1.GetOptions{})
	require.NoError(t, err)

	extraConfig, _, err := unstructured.NestedStringMap(argoCD.Object, "spec", "extraConfig")
	require.NoError(t, err)
	assert.Equal(t, "apiKey", extraConfig["accounts.octopus"])
	assert.Equal(t, "true", extraConfig["accounts.octopus.enabled"])

	policy, _, err := unstructured.NestedString(argoCD.Object, "spec", "rbac", "policy")
	require.NoError(t, err)
	assert.Contains(t, policy, "p, octopus, applications, get, *, allow")

	// The ConfigMap must be left alone; the operator owns it.
	cm, _, err := c.GetConfigMap(ctx, instance.Namespace, argocd.ConfigMapName)
	require.NoError(t, err)
	assert.NotContains(t, cm.Data, "accounts.octopus")
}

func TestConfigureAccount_OperatorManagedPreservesExistingConfig(t *testing.T) {
	ctx := context.Background()
	instance := operatorInstance()

	existing := argoCDResource("openshift-gitops", "openshift-gitops")
	require.NoError(t, unstructured.SetNestedStringMap(existing.Object,
		map[string]string{"timeout.reconciliation": "180s"}, "spec", "extraConfig"))
	require.NoError(t, unstructured.SetNestedField(existing.Object,
		"p, alice, applications, *, */*, allow\n", "spec", "rbac", "policy"))

	c := operatorCluster(existing)

	status, err := argocd.InspectAccount(ctx, c, instance, readOnlySpec())
	require.NoError(t, err)
	require.NoError(t, argocd.ConfigureAccount(ctx, c, instance, status))

	argoCD, err := c.Dynamic.Resource(instance.Operator.Resource).Namespace(instance.Namespace).
		Get(ctx, instance.Operator.Name, metav1.GetOptions{})
	require.NoError(t, err)

	extraConfig, _, _ := unstructured.NestedStringMap(argoCD.Object, "spec", "extraConfig")
	assert.Equal(t, "180s", extraConfig["timeout.reconciliation"])

	policy, _, _ := unstructured.NestedString(argoCD.Object, "spec", "rbac", "policy")
	assert.Contains(t, policy, "p, alice, applications, *, */*, allow")
	assert.Contains(t, policy, "p, octopus, clusters, get, *, allow")
}

// The operator regenerates argocd-secret, so a temporary password written there
// would be taken away, possibly mid-sign-in.
func TestBeginBootstrapLogin_RefusedForOperatorManagedArgoCD(t *testing.T) {
	c := operatorCluster(argoCDResource("openshift-gitops", "openshift-gitops"))

	_, err := argocd.BeginBootstrapLogin(context.Background(), c, operatorInstance(),
		argocd.AccountSpec{Name: "octopus"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "managed by an operator")
}
