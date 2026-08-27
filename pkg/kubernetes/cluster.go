package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Cluster struct {
	Clientset kubernetes.Interface
	// Dynamic reaches CRD-backed resources, such as Argo CD's AppProjects.
	Dynamic     dynamic.Interface
	ContextName string
	Server      string
}

func Connect(kubeConfig *KubeConfig, contextName string) (*Cluster, error) {
	restConfig, err := kubeConfig.RestConfig(contextName)
	if err != nil {
		return nil, err
	}
	return connectWithConfig(kubeConfig, contextName, restConfig)
}

func connectWithConfig(kubeConfig *KubeConfig, contextName string, restConfig *rest.Config) (*Cluster, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not build a Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not build a Kubernetes client: %w", err)
	}

	resolved := contextName
	if resolved == "" {
		if current, ok := kubeConfig.CurrentContext(); ok {
			resolved = current.Name
		}
	}

	return &Cluster{Clientset: clientset, Dynamic: dynamicClient, ContextName: resolved, Server: restConfig.Host}, nil
}

func NewClusterForTesting(clientset kubernetes.Interface, contextName, server string) *Cluster {
	return &Cluster{Clientset: clientset, ContextName: contextName, Server: server}
}

func (c *Cluster) WithDynamic(dynamicClient dynamic.Interface) *Cluster {
	c.Dynamic = dynamicClient
	return c
}

func (c *Cluster) ServerVersion() (string, error) {
	info, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("could not reach the Kubernetes API server: %w", err)
	}
	return info.GitVersion, nil
}

// NodeArchitectures is advisory, so a cluster that will not let the caller list
// nodes reports none rather than failing.
func (c *Cluster) NodeArchitectures(ctx context.Context) ([]string, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not list cluster nodes: %w", err)
	}

	seen := map[string]bool{}
	for _, n := range nodes.Items {
		if arch := n.Status.NodeInfo.Architecture; arch != "" {
			seen[arch] = true
		}
	}

	architectures := make([]string, 0, len(seen))
	for arch := range seen {
		architectures = append(architectures, arch)
	}
	sort.Strings(architectures)
	return architectures, nil
}

func (c *Cluster) NamespaceExists(ctx context.Context, name string) (bool, error) {
	_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, fmt.Errorf("could not check whether namespace %q exists: %w", name, err)
	}
}

func (c *Cluster) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, bool, error) {
	cm, err := c.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return cm, true, nil
	case apierrors.IsNotFound(err):
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("could not read ConfigMap %s/%s: %w", namespace, name, err)
	}
}

func (c *Cluster) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, bool, error) {
	s, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return s, true, nil
	case apierrors.IsNotFound(err):
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("could not read Secret %s/%s: %w", namespace, name, err)
	}
}

type Permission struct {
	Verb        string
	Group       string
	Resource    string
	Namespace   string
	Description string
}

func (p Permission) String() string {
	if p.Namespace != "" {
		return fmt.Sprintf("%s %s in namespace %s", p.Verb, p.Resource, p.Namespace)
	}
	return fmt.Sprintf("%s %s", p.Verb, p.Resource)
}

func InstallPermissions(namespace string) []Permission {
	return []Permission{
		{Verb: "create", Resource: "namespaces", Description: "create the install namespace"},
		{Verb: "create", Resource: "secrets", Namespace: namespace, Description: "store credentials for the chart"},
		{Verb: "create", Resource: "serviceaccounts", Namespace: namespace, Description: "create the chart's service account"},
		{Verb: "create", Group: "apps", Resource: "deployments", Namespace: namespace, Description: "deploy the chart's workloads"},
		{Verb: "create", Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Description: "grant the chart its cluster permissions"},
		{Verb: "create", Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Description: "bind the chart's cluster permissions"},
	}
}

// CheckPermissions returns the permissions that were denied. Checking up front
// means a missing one surfaces before the user answers a page of questions,
// rather than halfway through a partly applied install.
func (c *Cluster) CheckPermissions(ctx context.Context, permissions []Permission) ([]Permission, error) {
	var denied []Permission

	for _, p := range permissions {
		review := &authzv1.SelfSubjectAccessReview{
			Spec: authzv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzv1.ResourceAttributes{
					Namespace: p.Namespace,
					Verb:      p.Verb,
					Group:     p.Group,
					Resource:  p.Resource,
				},
			},
		}

		result, err := c.Clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("could not check whether you can %s: %w", p, err)
		}
		if !result.Status.Allowed {
			denied = append(denied, p)
		}
	}

	return denied, nil
}

func (c *Cluster) CreateNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

	_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create namespace %q: %w", name, err)
	}
	return nil
}

// UpsertSecret replaces a Secret's contents wholesale, dropping any key it was
// not given. Only use it for Secrets Octopus owns; for anything else use
// MergeSecretKeys.
func (c *Cluster) UpsertSecret(ctx context.Context, namespace, name string, data map[string]string) error {
	secrets := c.Clientset.CoreV1().Secrets(namespace)

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "octopus-cli"},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}

	existing, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil {
		return err
	}

	if !found {
		if _, err := secrets.Create(ctx, desired, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("could not create Secret %s/%s: %w", namespace, name, err)
		}
		return nil
	}

	desired.ResourceVersion = existing.ResourceVersion
	if _, err := secrets.Update(ctx, desired, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("could not update Secret %s/%s: %w", namespace, name, err)
	}
	return nil
}

// MergeSecretKeys is the only safe way to edit a Secret Octopus does not own.
// argocd-secret holds Argo CD's TLS and signing keys alongside anything Octopus
// puts there, and replacing it wholesale would destroy the installation.
func (c *Cluster) MergeSecretKeys(ctx context.Context, namespace, name string, set map[string]string, remove []string) error {
	secrets := c.Clientset.CoreV1().Secrets(namespace)

	existing, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Secret %s/%s does not exist", namespace, name)
	}

	updated := existing.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string][]byte{}
	}
	for key, value := range set {
		updated.Data[key] = []byte(value)
	}
	for _, key := range remove {
		delete(updated.Data, key)
	}

	// The resourceVersion that was read makes a concurrent change conflict
	// rather than be silently overwritten.
	if _, err := secrets.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("%s/%s changed while it was being updated; try again", namespace, name)
		}
		return fmt.Errorf("could not update Secret %s/%s: %w", namespace, name, err)
	}
	return nil
}

func (c *Cluster) SecretKey(ctx context.Context, namespace, name, key string) (string, bool, error) {
	secret, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil || !found {
		return "", false, err
	}
	value, ok := secret.Data[key]
	if !ok {
		return "", false, nil
	}
	return string(value), true, nil
}

func (c *Cluster) FindDeployment(ctx context.Context, namespace, selector string) (*appsv1.Deployment, bool, error) {
	list, err := c.Clientset.AppsV1().Deployments(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, false, fmt.Errorf("could not read deployments in namespace %s: %w", namespace, err)
	}
	if len(list.Items) == 0 {
		return nil, false, nil
	}
	return &list.Items[0], true, nil
}

// RestartDeployment rolls the pods so they pick up changed Secret values, which
// are only read at container start.
func (c *Cluster) RestartDeployment(ctx context.Context, namespace, name string) error {
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"octopus.com/restartedAt":%q}}}}}`,
		time.Now().UTC().Format(time.RFC3339))

	_, err := c.Clientset.AppsV1().Deployments(namespace).
		Patch(ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("could not restart deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}
