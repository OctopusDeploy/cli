package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

// HasAPIResource reports whether a CRD-backed resource is served by this
// cluster. Components detect each other this way rather than by looking for a
// Helm release, because a release can be named anything.
func (c *Cluster) HasAPIResource(group, resource string) (bool, error) {
	discovery := c.Clientset.Discovery()

	groups, err := discovery.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("could not list the API groups this cluster serves: %w", err)
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

// StorageClass is what an installer needs to know to choose where a component's
// volume comes from.
type StorageClass struct {
	Name        string
	Provisioner string
	IsDefault   bool
}

// readWriteManyProvisioners serve a shared filesystem, so many pods on many
// nodes can mount the same volume. Everything else provisions a block device
// that only one node can mount at a time.
//
// The Kubernetes API does not report which access modes a storage class
// supports, and this is the only signal it does give. An unrecognised
// provisioner is treated as one node at a time, because that costs nothing but
// node affinity, where guessing the other way leaves a volume that never binds.
var readWriteManyProvisioners = map[string]bool{
	"efs.csi.aws.com":                             true, // AWS EFS
	"filestore.csi.storage.gke.io":                true, // Google Filestore
	"file.csi.azure.com":                          true, // Azure Files
	"kubernetes.io/azure-file":                    true,
	"nfs.csi.k8s.io":                              true,
	"smb.csi.k8s.io":                              true,
	"cephfs.csi.ceph.com":                         true,
	"rook-ceph.cephfs.csi.ceph.com":               true,
	"openebs.io/nfsrwx":                           true,
	"nfs.openebs.io":                              true,
	"k8s-sigs.io/nfs-subdir-external-provisioner": true,
}

// SupportsReadWriteMany reports whether a volume from this class can be mounted
// by pods on more than one node.
func (s StorageClass) SupportsReadWriteMany() bool {
	return readWriteManyProvisioners[s.Provisioner]
}

func (s StorageClass) Display() string {
	if s.IsDefault {
		return fmt.Sprintf("%s (cluster default, %s)", s.Name, s.Provisioner)
	}
	return fmt.Sprintf("%s (%s)", s.Name, s.Provisioner)
}

// StorageClasses is advisory, so a cluster that will not let the caller list
// them reports none rather than failing.
func (c *Cluster) StorageClasses(ctx context.Context) ([]StorageClass, error) {
	list, err := c.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not list the cluster's storage classes: %w", err)
	}

	classes := make([]StorageClass, 0, len(list.Items))
	for _, item := range list.Items {
		classes = append(classes, StorageClass{
			Name:        item.Name,
			Provisioner: item.Provisioner,
			IsDefault:   item.Annotations["storageclass.kubernetes.io/is-default-class"] == "true",
		})
	}

	sort.Slice(classes, func(i, j int) bool {
		if classes[i].IsDefault != classes[j].IsDefault {
			return classes[i].IsDefault
		}
		return classes[i].Name < classes[j].Name
	})
	return classes, nil
}

// Role is an existing role whose rules can be copied into a component's own
// role. Namespace is empty for a cluster role.
type Role struct {
	Name      string
	Namespace string
	Rules     []rbacv1.PolicyRule
}

func (r Role) IsClusterScoped() bool { return r.Namespace == "" }

// Reference is how a role is named on a command line: a bare name for a cluster
// role, and namespace/name for one that lives in a namespace.
func (r Role) Reference() string {
	if r.IsClusterScoped() {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}

// GrantsEverything reports an unrestricted role, which is worth saying out loud
// before somebody copies it expecting to have restricted something.
func (r Role) GrantsEverything() bool {
	for _, rule := range r.Rules {
		if contains(rule.Verbs, "*") && contains(rule.APIGroups, "*") && contains(rule.Resources, "*") {
			return true
		}
	}
	return false
}

func (r Role) Display() string {
	kind := "role"
	if r.IsClusterScoped() {
		kind = "cluster role"
	}

	if r.GrantsEverything() {
		return fmt.Sprintf("%s (%s, full access to the cluster)", r.Reference(), kind)
	}
	return fmt.Sprintf("%s (%s, %d %s)", r.Reference(), kind, len(r.Rules), Pluralise("rule", "rules", len(r.Rules)))
}

// Roles lists the roles worth offering to copy, cluster-scoped ones first.
// Kubernetes ships around seventy of its own, all prefixed system:, and the
// kube- namespaces hold the control plane's, none of which is what anybody is
// looking for here.
func (c *Cluster) Roles(ctx context.Context) ([]Role, error) {
	clusterRoles, err := c.Clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list the cluster's roles: %w", err)
	}

	roles := make([]Role, 0, len(clusterRoles.Items))
	for _, item := range clusterRoles.Items {
		if strings.HasPrefix(item.Name, "system:") {
			continue
		}
		roles = append(roles, Role{Name: item.Name, Rules: item.Rules})
	}

	namespaced, err := c.Clientset.RbacV1().Roles(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		// A credential that can read cluster roles but not namespaced ones still
		// has something to offer.
		if !apierrors.IsForbidden(err) {
			return nil, fmt.Errorf("could not list the cluster's roles: %w", err)
		}
	} else {
		for _, item := range namespaced.Items {
			if strings.HasPrefix(item.Namespace, "kube-") || strings.HasPrefix(item.Name, "system:") {
				continue
			}
			roles = append(roles, Role{Name: item.Name, Namespace: item.Namespace, Rules: item.Rules})
		}
	}

	sort.Slice(roles, func(i, j int) bool {
		if roles[i].IsClusterScoped() != roles[j].IsClusterScoped() {
			return roles[i].IsClusterScoped()
		}
		return roles[i].Reference() < roles[j].Reference()
	})
	return roles, nil
}

// FindRole reads one role by the reference a command line gives it.
func (c *Cluster) FindRole(ctx context.Context, reference string) (Role, error) {
	namespace, name, namespaced := strings.Cut(strings.TrimSpace(reference), "/")
	if !namespaced {
		role, err := c.Clientset.RbacV1().ClusterRoles().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return Role{}, fmt.Errorf("this cluster has no cluster role named %q. Name a role in a namespace as namespace/name", reference)
			}
			return Role{}, fmt.Errorf("could not read cluster role %q: %w", reference, err)
		}
		return Role{Name: role.Name, Rules: role.Rules}, nil
	}

	role, err := c.Clientset.RbacV1().Roles(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return Role{}, fmt.Errorf("namespace %s has no role named %q", namespace, name)
		}
		return Role{}, fmt.Errorf("could not read role %q: %w", reference, err)
	}
	return Role{Name: role.Name, Namespace: role.Namespace, Rules: role.Rules}, nil
}

// MergePolicyRules gathers the rules of several roles into one list. RBAC is
// additive, so a workload given all of them ends up with the union; exact
// duplicates only make the result harder to read.
func MergePolicyRules(roles []Role) []any {
	merged := make([]any, 0)
	seen := map[string]bool{}

	for _, role := range roles {
		for _, value := range PolicyRuleValues(role.Rules) {
			key := fmt.Sprint(value)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, value)
		}
	}
	return merged
}

// PolicyRuleValues converts RBAC rules into the plain maps a Helm value has to
// be. The chart checks the value is a list and renders it with toYaml, neither
// of which a typed struct survives.
func PolicyRuleValues(rules []rbacv1.PolicyRule) []any {
	values := make([]any, 0, len(rules))
	for _, rule := range rules {
		value := map[string]any{}
		addStrings(value, "apiGroups", rule.APIGroups)
		addStrings(value, "resources", rule.Resources)
		addStrings(value, "resourceNames", rule.ResourceNames)
		addStrings(value, "nonResourceURLs", rule.NonResourceURLs)
		addStrings(value, "verbs", rule.Verbs)
		if len(value) > 0 {
			values = append(values, value)
		}
	}
	return values
}

func addStrings(value map[string]any, key string, values []string) {
	if len(values) > 0 {
		value[key] = values
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
