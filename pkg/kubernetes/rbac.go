package kubernetes

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/util"
	authzv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		if slices.Contains(rule.Verbs, "*") && slices.Contains(rule.APIGroups, "*") && slices.Contains(rule.Resources, "*") {
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
	return fmt.Sprintf("%s (%s, %d %s)", r.Reference(), kind, len(r.Rules), util.Pluralise("rule", "rules", len(r.Rules)))
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
