// Package argocd discovers an Argo CD installation in a cluster and, on
// request, prepares the Octopus account the gateway authenticates as.
package argocd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

const (
	// ConfigMapName is also how an installation is recognised when its labels
	// do not match anything expected: Argo CD reads its configuration from a
	// ConfigMap of exactly this name, so every installation has one.

	ConfigMapName       = "argocd-cm"
	RBACConfigMapName   = "argocd-rbac-cm"
	ParamsConfigMapName = "argocd-cmd-params-cm"

	// Argo CD generates this at install time, and tells administrators to
	// delete it once they have logged in.
	InitialAdminSecretName = "argocd-initial-admin-secret"

	// Holds Argo CD's TLS and session signing keys alongside local account
	// credentials, so it must only ever be edited key by key, never replaced.
	SecretName = "argocd-secret"
)

const (
	capabilityAPIKey = "apiKey"
	capabilityLogin  = "login"
)

// OperatorInstance is the ArgoCD custom resource an operator manages an
// installation from, as used by the Argo CD operator and OpenShift GitOps.
type OperatorInstance struct {
	Name     string
	Resource schema.GroupVersionResource
}

// Kind changes almost every connection setting the gateway needs.
type Kind string

const (
	KindInCluster Kind = "in-cluster"
	// AWS runs Argo CD in its own control plane and exposes it publicly.
	KindEKSManaged Kind = "eks-managed"
)

type Instance struct {
	Kind Kind

	// Name is set where an instance has its own, such as the EKS capability
	// name. In-cluster installs are identified by their namespace instead.
	Name string
	// Status is reported by managed instances only.
	Status string
	// Operator names the resource an operator reconciles this instance from,
	// when one does. Argo CD's ConfigMaps are then generated, so changes
	// written straight to them are reverted.
	Operator *OperatorInstance

	Namespace     string
	ServiceName   string
	Version       string
	ServerGRPCURL string
	// Plaintext means Argo CD serves the API without TLS, which is what
	// `server.insecure` does. Maps to gateway.argocd.plaintext.
	Plaintext bool
	// SelfSignedTLS maps to gateway.argocd.insecure.
	SelfSignedTLS bool
	// GRPCWeb tunnels gRPC over HTTP/1.1, which AWS's load balancer requires
	// because it does not speak HTTP/2.
	GRPCWeb         bool
	GRPCWebRootPath string
	WebUIURL        string
}

// IsManaged means Argo CD is hosted outside the cluster, so the account and
// RBAC automation does not apply.
func (i Instance) IsManaged() bool {
	return i.Kind == KindEKSManaged
}

func (i Instance) Display() string {
	if i.IsManaged() {
		s := "AWS managed Argo CD"
		if i.Name != "" {
			s = fmt.Sprintf("%s %s", s, i.Name)
		}
		if i.Version != "" {
			s = fmt.Sprintf("%s, %s", s, i.Version)
		}
		return fmt.Sprintf("%s (%s)", s, i.ServerGRPCURL)
	}

	s := fmt.Sprintf("%s (namespace %s)", i.ServiceName, i.Namespace)
	if i.Version != "" {
		s = fmt.Sprintf("%s, %s", s, i.Version)
	}
	return s
}

type ErrNoInstances struct {
	// Skipped explains any candidate that was found but could not be used.
	Skipped []string
}

func (e ErrNoInstances) Error() string {
	message := "no Argo CD API server was found in this cluster. Octopus looked for a deployment labelled as one, and in every " +
		"namespace holding an " + ConfigMapName + " ConfigMap. If Argo CD is there under different labels, name its namespace with " +
		"--argocd-namespace; if it runs outside this cluster, give its address with --argocd-server-grpc-url. Note that an Argo CD " +
		"running in core mode has no API server for the gateway to connect to"

	if len(e.Skipped) > 0 {
		message += "\n\nThese were found but could not be used:\n  " + strings.Join(e.Skipped, "\n  ")
	}
	return message
}

// argoCDSelectors find resources belonging to an Argo CD installation. Every
// one names Argo CD: a selector like component=server on its own matches any
// application in the cluster that happens to have a server.
var argoCDSelectors = []string{
	"app.kubernetes.io/part-of=argocd",
	"app.kubernetes.io/name=argocd-server",
	"app=argocd-server",
}

// nonAPIServerComponents are Argo CD's other workloads. They carry the same
// part-of label as the API server, and some of them are also named "-server".
var nonAPIServerComponents = []string{
	"repo-server", "dex-server", "redis", "application-controller",
	"applicationset-controller", "notifications-controller", "commit-server",
}

func Discover(ctx context.Context, c *octoK8s.Cluster) ([]Instance, error) {
	deployments, err := findServerDeployments(ctx, c)
	if err != nil {
		return nil, err
	}

	instances := make([]Instance, 0, len(deployments))
	var skipped []string
	for i := range deployments {
		d := &deployments[i]

		instance, err := describe(ctx, c, d.Namespace, d.Name, imageOf(d.Spec.Template.Spec.Containers))
		if err != nil {
			// One unusable candidate must not hide a working installation
			// elsewhere in the cluster, but the reason is worth keeping in case
			// it turns out to be the only one.
			skipped = append(skipped, err.Error())
			continue
		}
		instances = append(instances, instance)
	}

	if len(instances) == 0 {
		return nil, ErrNoInstances{Skipped: skipped}
	}

	sort.Slice(instances, func(i, j int) bool { return instances[i].Namespace < instances[j].Namespace })
	return instances, nil
}

// DiscoverInNamespace finds Argo CD in one namespace, for when its labels are
// not what any of the selectors expect and the namespace was given explicitly.
func DiscoverInNamespace(ctx context.Context, c *octoK8s.Cluster, namespace string) (Instance, error) {
	deployments, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Instance{}, fmt.Errorf("could not read deployments in namespace %s: %w", namespace, err)
	}

	for i := range deployments.Items {
		d := &deployments.Items[i]
		if !isAPIServer(d) {
			continue
		}
		return describe(ctx, c, d.Namespace, d.Name, imageOf(d.Spec.Template.Spec.Containers))
	}

	return Instance{}, fmt.Errorf(
		"namespace %s does not contain an Argo CD API server. The gateway connects to Argo CD's API server, "+
			"so it cannot be used with an Argo CD running in core mode", namespace)
}

func findServerDeployments(ctx context.Context, c *octoK8s.Cluster) ([]appsv1.Deployment, error) {
	seen := map[string]bool{}
	var found []appsv1.Deployment

	add := func(d appsv1.Deployment) {
		key := d.Namespace + "/" + d.Name
		if !seen[key] && isAPIServer(&d) {
			seen[key] = true
			found = append(found, d)
		}
	}

	for _, selector := range argoCDSelectors {
		list, err := c.Clientset.AppsV1().Deployments(metav1.NamespaceAll).
			List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, fmt.Errorf("could not search the cluster for Argo CD: %w", err)
		}
		for i := range list.Items {
			add(list.Items[i])
		}
	}

	if len(found) > 0 {
		return found, nil
	}

	// Nothing carried a label naming Argo CD, so fall back to the namespaces
	// that hold its ConfigMap and require the workload to run an Argo CD image.
	for _, namespace := range namespacesWithArgoConfig(ctx, c) {
		list, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for i := range list.Items {
			if runsArgoCD(&list.Items[i]) {
				add(list.Items[i])
			}
		}
	}

	return found, nil
}

// isAPIServer distinguishes the API server from Argo CD's other workloads,
// which share its labels and are sometimes also named "-server".
func isAPIServer(d *appsv1.Deployment) bool {
	if component := d.Labels["app.kubernetes.io/component"]; component != "" {
		return component == "server"
	}

	name := d.Name
	if !strings.HasSuffix(name, "-server") {
		return false
	}
	for _, other := range nonAPIServerComponents {
		if strings.HasSuffix(name, other) {
			return false
		}
	}
	return true
}

func runsArgoCD(d *appsv1.Deployment) bool {
	for _, container := range d.Spec.Template.Spec.Containers {
		if strings.Contains(container.Image, "argocd") || strings.Contains(container.Image, "argo-cd") {
			return true
		}
	}
	return false
}

// namespacesWithArgoConfig finds installations whose labels name nothing
// recognisable. Argo CD reads its configuration from a ConfigMap of a fixed
// name, so the namespaces holding one are where to look.
func namespacesWithArgoConfig(ctx context.Context, c *octoK8s.Cluster) []string {
	namespaces, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var found []string
	for _, namespace := range namespaces.Items {
		if _, ok, err := c.GetConfigMap(ctx, namespace.Name, ConfigMapName); err == nil && ok {
			found = append(found, namespace.Name)
		}
	}
	return found
}

func describe(ctx context.Context, c *octoK8s.Cluster, namespace, deploymentName, image string) (Instance, error) {
	instance := Instance{
		Kind:        KindInCluster,
		Namespace:   namespace,
		ServiceName: deploymentName,
		Version:     versionFromImage(image),
	}

	service, err := c.Clientset.CoreV1().Services(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return Instance{}, fmt.Errorf("found an Argo CD API server in namespace %s but could not read its Service %q: %w", namespace, deploymentName, err)
	}

	// `server.insecure` makes Argo CD serve the API without TLS. Otherwise it
	// serves TLS, by default with a self-signed certificate - both of which the
	// gateway has to be told about explicitly or it will refuse to connect.
	instance.Plaintext = isInsecureMode(ctx, c, namespace)
	instance.SelfSignedTLS = !instance.Plaintext

	instance.ServerGRPCURL = grpcURL(service, instance.Plaintext)
	instance.WebUIURL = webUIURL(ctx, c, namespace, service)
	instance.Operator = findOperatorInstance(ctx, c, namespace)

	return instance, nil
}

// grpcURL is always a Service DNS name: the gateway runs inside the cluster.
func grpcURL(service *corev1.Service, plaintext bool) string {
	port := servicePort(service, plaintext)
	host := fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)

	// Default ports carry no information, so leave them off.
	if (plaintext && port == 80) || (!plaintext && port == 443) {
		return "grpc://" + host
	}
	return fmt.Sprintf("grpc://%s:%d", host, port)
}

func servicePort(service *corev1.Service, plaintext bool) int32 {
	wanted := "https"
	fallback := int32(443)
	if plaintext {
		wanted, fallback = "http", 80
	}

	for _, p := range service.Spec.Ports {
		if p.Name == wanted {
			return p.Port
		}
	}
	for _, p := range service.Spec.Ports {
		if p.Port == fallback {
			return p.Port
		}
	}
	if len(service.Spec.Ports) > 0 {
		return service.Spec.Ports[0].Port
	}
	return fallback
}

func isInsecureMode(ctx context.Context, c *octoK8s.Cluster, namespace string) bool {
	cm, found, err := c.GetConfigMap(ctx, namespace, ParamsConfigMapName)
	if err != nil || !found {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cm.Data["server.insecure"]), "true")
}

// webUIURL is a convenience for linking from Octopus, so every lookup degrades
// to an empty string.
func webUIURL(ctx context.Context, c *octoK8s.Cluster, namespace string, service *corev1.Service) string {
	if cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName); err == nil && found {
		if url := strings.TrimSpace(cm.Data["url"]); url != "" {
			return url
		}
	}

	if url := ingressURL(ctx, c, namespace, service.Name); url != "" {
		return url
	}

	return loadBalancerURL(service)
}

func ingressURL(ctx context.Context, c *octoK8s.Cluster, namespace, serviceName string) string {
	ingresses, err := c.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ""
	}

	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			if rule.Host == "" || rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && path.Backend.Service.Name == serviceName {
					return "https://" + rule.Host
				}
			}
		}
	}
	return ""
}

func loadBalancerURL(service *corev1.Service) string {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return ""
	}
	for _, ing := range service.Status.LoadBalancer.Ingress {
		if ing.Hostname != "" {
			return "https://" + ing.Hostname
		}
		if ing.IP != "" {
			return "https://" + ing.IP
		}
	}
	return ""
}

func imageOf(containers []corev1.Container) string {
	for _, c := range containers {
		if strings.Contains(c.Image, "argocd") {
			return c.Image
		}
	}
	if len(containers) > 0 {
		return containers[0].Image
	}
	return ""
}

// versionFromImage turns quay.io/argoproj/argocd:v3.4.2 into v3.4.2.
func versionFromImage(image string) string {
	if image == "" {
		return ""
	}
	// Strip any digest first so the tag search cannot hit it.
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	colon := strings.LastIndex(image, ":")
	if colon < 0 {
		return ""
	}
	// A colon before the last slash is a registry port, not a tag.
	if strings.Contains(image[colon:], "/") {
		return ""
	}
	return image[colon+1:]
}

// argoCDResources are the ArgoCD custom resource versions an operator may use.
var argoCDResources = []schema.GroupVersionResource{
	{Group: "argoproj.io", Version: "v1beta1", Resource: "argocds"},
	{Group: "argoproj.io", Version: "v1alpha1", Resource: "argocds"},
}

// findOperatorInstance looks for the resource an operator reconciles this
// installation from. A cluster without the CRD simply has none.
func findOperatorInstance(ctx context.Context, c *octoK8s.Cluster, namespace string) *OperatorInstance {
	if c.Dynamic == nil {
		return nil
	}

	for _, gvr := range argoCDResources {
		list, err := c.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil || len(list.Items) == 0 {
			continue
		}
		return &OperatorInstance{Name: list.Items[0].GetName(), Resource: gvr}
	}
	return nil
}
