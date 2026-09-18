package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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

	// apiGroups caches the discovery listing: it is a full API-group round
	// trip, and one wizard run asks HasAPIResource for several components.
	apiGroups *metav1.APIGroupList
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

func (c *Cluster) CreateNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

	_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create namespace %q: %w", name, err)
	}
	return nil
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

// HasAPIResource reports whether a CRD-backed resource is served by this
// cluster. Components detect each other this way rather than by looking for a
// Helm release, because a release can be named anything.
func (c *Cluster) HasAPIResource(group, resource string) (bool, error) {
	discovery := c.Clientset.Discovery()

	groups := c.apiGroups
	if groups == nil {
		listed, err := discovery.ServerGroups()
		if err != nil {
			return false, fmt.Errorf("could not list the API groups this cluster serves: %w", err)
		}
		c.apiGroups = listed
		groups = listed
	}

	for _, g := range groups.Groups {
		if g.Name != group {
			continue
		}
		for _, version := range g.Versions {
			list, err := discovery.ServerResourcesForGroupVersion(version.GroupVersion)
			if err != nil {
				// A group can advertise a version whose resources cannot be
				// listed, usually an aggregated API server that is down. Other
				// versions may still answer.
				continue
			}
			for _, r := range list.APIResources {
				if r.Name == resource {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
