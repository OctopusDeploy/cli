package argocd_test

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// argoCDLabels are what every Argo CD install puts on its API server. The name
// varies with how it was installed - an operator names resources after its
// ArgoCD resource - so discovery matches on part-of and component instead.
func argoCDLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      name,
		"app.kubernetes.io/part-of":   "argocd",
		"app.kubernetes.io/component": "server",
	}
}

func serverDeployment(namespace, image string) *appsv1.Deployment {
	return namedServerDeployment(namespace, "argocd-server", image)
}

func namedServerDeployment(namespace, name, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    argoCDLabels(name),
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "argocd-server", Image: image}}},
			},
		},
	}
}

// serverService mirrors the Service a stock Argo CD install creates.
func serverService(namespace string) *corev1.Service {
	return namedServerService(namespace, "argocd-server")
}

func namedServerService(namespace, name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: argoCDLabels(name)},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80},
				{Name: "https", Port: 443},
			},
		},
	}
}

func clusterWith(objects ...runtime.Object) *octoK8s.Cluster {
	return octoK8s.NewClusterForTesting(fake.NewSimpleClientset(objects...), "test", "https://cluster")
}

func TestDiscover_StockInstall(t *testing.T) {
	c := clusterWith(
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)

	got := instances[0]
	assert.Equal(t, "argocd", got.Namespace)
	assert.Equal(t, "argocd-server", got.ServiceName)
	assert.Equal(t, "v3.4.2", got.Version)
	// A stock install serves TLS with a self-signed certificate, so the gateway
	// has to be told to skip verification but keep TLS on.
	assert.Equal(t, "grpc://argocd-server.argocd.svc.cluster.local", got.ServerGRPCURL)
	assert.False(t, got.Plaintext)
	assert.True(t, got.SelfSignedTLS)
}

func TestDiscover_InsecureMode(t *testing.T) {
	c := clusterWith(
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: argocd.ParamsConfigMapName, Namespace: "argocd"},
			Data:       map[string]string{"server.insecure": "true"},
		},
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)

	assert.True(t, instances[0].Plaintext)
	assert.False(t, instances[0].SelfSignedTLS)
	assert.Equal(t, "grpc://argocd-server.argocd.svc.cluster.local", instances[0].ServerGRPCURL)
}

func TestDiscover_NonDefaultPort(t *testing.T) {
	svc := serverService("argocd")
	svc.Spec.Ports = []corev1.ServicePort{{Name: "https", Port: 8443}}

	c := clusterWith(serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"), svc)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "grpc://argocd-server.argocd.svc.cluster.local:8443", instances[0].ServerGRPCURL)
}

func TestDiscover_NoArgoCD(t *testing.T) {
	_, err := argocd.Discover(context.Background(), clusterWith())
	assert.ErrorAs(t, err, &argocd.ErrNoInstances{})
}

func TestDiscover_MultipleInstancesSortedByNamespace(t *testing.T) {
	c := clusterWith(
		serverDeployment("team-b", "quay.io/argoproj/argocd:v3.4.2"), serverService("team-b"),
		serverDeployment("team-a", "quay.io/argoproj/argocd:v3.3.0"), serverService("team-a"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 2)
	assert.Equal(t, "team-a", instances[0].Namespace)
	assert.Equal(t, "team-b", instances[1].Namespace)
}

func TestDiscover_WebUIURLFromConfigMap(t *testing.T) {
	c := clusterWith(
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: argocd.ConfigMapName, Namespace: "argocd"},
			Data:       map[string]string{"url": "https://argo.example.com"},
		},
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "https://argo.example.com", instances[0].WebUIURL)
}

func TestDiscover_WebUIURLFromIngress(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	c := clusterWith(
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: "argocd"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "argo.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{Name: "argocd-server"},
								},
							}},
						},
					},
				}},
			},
		},
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "https://argo.example.com", instances[0].WebUIURL)
}

func TestDiscover_NoWebUIURLIsNotAnError(t *testing.T) {
	c := clusterWith(serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"), serverService("argocd"))

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	assert.Empty(t, instances[0].WebUIURL)
}

// When the only candidate cannot be used, its reason must survive rather than
// being reported as though nothing was there.
func TestDiscover_ReportsWhyTheOnlyCandidateWasUnusable(t *testing.T) {
	c := clusterWith(serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"))

	_, err := argocd.Discover(context.Background(), c)
	require.Error(t, err)
	assert.ErrorAs(t, err, &argocd.ErrNoInstances{})
	assert.Contains(t, err.Error(), "could not read its Service")
}

// OpenShift GitOps and the Argo CD operator name every resource after their
// ArgoCD resource, so nothing is called argocd-server.
func TestDiscover_OperatorNamedInstall(t *testing.T) {
	c := clusterWith(
		namedServerDeployment("openshift-gitops", "openshift-gitops-server", "quay.io/argoproj/argocd:v3.4.2"),
		namedServerService("openshift-gitops", "openshift-gitops-server"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)

	assert.Equal(t, "openshift-gitops", instances[0].Namespace)
	assert.Equal(t, "openshift-gitops-server", instances[0].ServiceName)
	assert.Equal(t, "grpc://openshift-gitops-server.openshift-gitops.svc.cluster.local", instances[0].ServerGRPCURL)
}

// An installation whose labels match nothing expected is still found, because
// Argo CD reads its configuration from a ConfigMap of a fixed name.
func TestDiscover_FallsBackToTheArgoCDConfigMap(t *testing.T) {
	unlabelled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "gitops-server", Namespace: "tools"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "quay.io/argoproj/argocd:v3.4.2"}}},
			},
		},
	}

	c := clusterWith(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tools"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: argocd.ConfigMapName, Namespace: "tools"}},
		unlabelled,
		namedServerService("tools", "gitops-server"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "gitops-server", instances[0].ServiceName)
}

// Argo CD's other workloads share the API server's labels, so the repo server
// must not be mistaken for it.
func TestDiscover_IgnoresTheRepoServer(t *testing.T) {
	repoServer := namedServerDeployment("argocd", "argocd-repo-server", "quay.io/argoproj/argocd:v3.4.2")
	repoServer.Labels["app.kubernetes.io/component"] = "repo-server"

	c := clusterWith(
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
		repoServer,
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "argocd-server", instances[0].ServiceName)
}

// The same installation matching more than one selector must not appear twice.
func TestDiscover_DoesNotDuplicateAcrossSelectors(t *testing.T) {
	c := clusterWith(serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"), serverService("argocd"))

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	assert.Len(t, instances, 1)
}

func TestDiscoverInNamespace(t *testing.T) {
	c := clusterWith(
		namedServerDeployment("gitops", "my-argo-server", "quay.io/argoproj/argocd:v3.4.2"),
		namedServerService("gitops", "my-argo-server"),
	)

	instance, err := argocd.DiscoverInNamespace(context.Background(), c, "gitops")
	require.NoError(t, err)
	assert.Equal(t, "my-argo-server", instance.ServiceName)

	_, err = argocd.DiscoverInNamespace(context.Background(), c, "empty")
	assert.ErrorContains(t, err, "does not contain an Argo CD API server")
}

// A selector like component=server matches anything in the cluster with a
// server, so an unrelated application must not be mistaken for Argo CD.
func TestDiscover_IgnoresUnrelatedServers(t *testing.T) {
	unrelated := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "model-catalog-server",
			Namespace: "kubeflow",
			Labels: map[string]string{
				"app.kubernetes.io/name":      "model-catalog",
				"app.kubernetes.io/component": "server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "kubeflow/model-catalog:1.0"}}},
			},
		},
	}

	c := clusterWith(
		unrelated,
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "argocd", instances[0].Namespace)
}

// An unrelated deployment with no Service of its own must not abort the search
// before the real Argo CD is reached.
func TestDiscover_OneUnusableCandidateDoesNotHideTheRest(t *testing.T) {
	strayArgo := namedServerDeployment("stray", "argocd-server", "quay.io/argoproj/argocd:v3.4.2")

	c := clusterWith(
		strayArgo, // deliberately has no Service
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
	)

	instances, err := argocd.Discover(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "argocd", instances[0].Namespace)
}

// Argo CD's other workloads carry the same part-of label, and dex is also
// named "-server".
func TestDiscover_IgnoresArgoCDsOtherWorkloads(t *testing.T) {
	objects := []runtime.Object{
		serverDeployment("argocd", "quay.io/argoproj/argocd:v3.4.2"),
		serverService("argocd"),
	}
	for _, name := range []string{"argocd-repo-server", "argocd-dex-server", "argocd-applicationset-controller"} {
		d := namedServerDeployment("argocd", name, "quay.io/argoproj/argocd:v3.4.2")
		delete(d.Labels, "app.kubernetes.io/component")
		objects = append(objects, d)
	}

	instances, err := argocd.Discover(context.Background(), clusterWith(objects...))
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "argocd-server", instances[0].ServiceName)
}

// The ConfigMap fallback must not sweep up whatever else lives in that
// namespace.
func TestDiscover_ConfigMapFallbackRequiresAnArgoCDImage(t *testing.T) {
	notArgo := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "some-server", Namespace: "tools"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "nginx:1.27"}}},
			},
		},
	}

	c := clusterWith(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tools"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: argocd.ConfigMapName, Namespace: "tools"}},
		notArgo,
	)

	_, err := argocd.Discover(context.Background(), c)
	assert.ErrorAs(t, err, &argocd.ErrNoInstances{})
}
