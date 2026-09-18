package argocd

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

// AppProjects live in the cluster even when Argo CD itself runs outside it,
// which is what makes them reachable for the AWS managed capability.
var appProjectGVR = schema.GroupVersionResource{
	Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects",
}

type Project struct {
	Name  string
	Roles []string
}

func (p Project) Display() string {
	if len(p.Roles) == 0 {
		return p.Name
	}
	return fmt.Sprintf("%s (roles: %s)", p.Name, strings.Join(p.Roles, ", "))
}

// ListProjects returns none rather than failing when Argo CD's CRDs are absent
// or unreadable, since that only means there is nothing to offer.
func ListProjects(ctx context.Context, c *octoK8s.Cluster, namespace string) ([]Project, error) {
	if c.Dynamic == nil {
		return nil, nil
	}

	list, err := c.Dynamic.Resource(appProjectGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not list Argo CD projects in namespace %s: %w", namespace, err)
	}

	projects := make([]Project, 0, len(list.Items))
	for i := range list.Items {
		projects = append(projects, Project{
			Name:  list.Items[i].GetName(),
			Roles: roleNames(&list.Items[i]),
		})
	}

	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

type ProjectRoleSpec struct {
	Project   string
	Role      string
	AllowSync bool
}

// Policies are scoped to the project, because a project role token cannot be
// granted anything outside it. Cluster resources are deliberately absent: they
// are not project-scoped, so a project role can never read them.
func (s ProjectRoleSpec) Policies() []string {
	subject := fmt.Sprintf("proj:%s:%s", s.Project, s.Role)

	policies := []string{
		fmt.Sprintf("p, %s, applications, get, %s/*, allow", subject, s.Project),
		fmt.Sprintf("p, %s, logs, get, %s/*, allow", subject, s.Project),
	}
	if s.AllowSync {
		policies = append(policies, fmt.Sprintf("p, %s, applications, sync, %s/*, allow", subject, s.Project))
	}
	sort.Strings(policies)
	return policies
}

type ProjectRoleStatus struct {
	Spec            ProjectRoleSpec
	Exists          bool
	MissingPolicies []string
}

func (s ProjectRoleStatus) IsComplete() bool {
	return s.Exists && len(s.MissingPolicies) == 0
}

func InspectProjectRole(ctx context.Context, c *octoK8s.Cluster, namespace string, spec ProjectRoleSpec) (ProjectRoleStatus, error) {
	status := ProjectRoleStatus{Spec: spec, MissingPolicies: spec.Policies()}

	project, err := getProject(ctx, c, namespace, spec.Project)
	if err != nil {
		return ProjectRoleStatus{}, err
	}

	role, found := findRole(project, spec.Role)
	if !found {
		return status, nil
	}

	status.Exists = true
	status.MissingPolicies = missingPolicies(strings.Join(stringSlice(role, "policies"), "\n"), spec.Policies())
	return status, nil
}

// ProjectRolePatchPlan shows what EnsureProjectRole would add, for the user to
// agree to first.
func ProjectRolePatchPlan(namespace string, statuses []ProjectRoleStatus) string {
	var b strings.Builder

	for _, status := range statuses {
		if status.IsComplete() {
			continue
		}

		fmt.Fprintf(&b, "  %s appproject/%s\n", namespace, status.Spec.Project)
		if !status.Exists {
			fmt.Fprintf(&b, "    + role %s\n", status.Spec.Role)
		}
		for _, p := range status.MissingPolicies {
			fmt.Fprintf(&b, "      + %s\n", p)
		}
	}

	return b.String()
}

// EnsureProjectRole leaves every other role on the project alone.
func EnsureProjectRole(ctx context.Context, c *octoK8s.Cluster, namespace string, status ProjectRoleStatus) error {
	if status.IsComplete() {
		return nil
	}

	project, err := getProject(ctx, c, namespace, status.Spec.Project)
	if err != nil {
		return err
	}

	roles := roleList(project)
	index := -1
	for i, role := range roles {
		if name, _ := role["name"].(string); name == status.Spec.Role {
			index = i
			break
		}
	}

	if index < 0 {
		roles = append(roles, map[string]any{
			"name":        status.Spec.Role,
			"description": "Read access for Octopus Deploy",
			"policies":    toAnySlice(status.Spec.Policies()),
		})
	} else {
		roles[index]["policies"] = toAnySlice(append(stringSlice(roles[index], "policies"), status.MissingPolicies...))
	}

	if err := unstructured.SetNestedSlice(project.Object, toAnySliceOfMaps(roles), "spec", "roles"); err != nil {
		return fmt.Errorf("could not update Argo CD project %s: %w", status.Spec.Project, err)
	}

	_, err = c.Dynamic.Resource(appProjectGVR).Namespace(namespace).Update(ctx, project, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("Argo CD project %s changed while it was being updated; try again", status.Spec.Project)
		}
		return fmt.Errorf("could not add the %s role to Argo CD project %s: %w", status.Spec.Role, status.Spec.Project, err)
	}
	return nil
}

func getProject(ctx context.Context, c *octoK8s.Cluster, namespace, name string) (*unstructured.Unstructured, error) {
	if c.Dynamic == nil {
		return nil, fmt.Errorf("no Kubernetes client is available to read Argo CD projects")
	}

	project, err := c.Dynamic.Resource(appProjectGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not read Argo CD project %s in namespace %s: %w", name, namespace, err)
	}
	return project, nil
}

func findRole(project *unstructured.Unstructured, name string) (map[string]any, bool) {
	for _, role := range roleList(project) {
		if roleName, _ := role["name"].(string); roleName == name {
			return role, true
		}
	}
	return nil, false
}

func roleNames(project *unstructured.Unstructured) []string {
	var names []string
	for _, role := range roleList(project) {
		if name, _ := role["name"].(string); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func roleList(project *unstructured.Unstructured) []map[string]any {
	raw, found, err := unstructured.NestedSlice(project.Object, "spec", "roles")
	if err != nil || !found {
		return nil
	}

	roles := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if role, ok := item.(map[string]any); ok {
			roles = append(roles, role)
		}
	}
	return roles
}

func stringSlice(object map[string]any, field string) []string {
	raw, ok := object[field].([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			values = append(values, s)
		}
	}
	return values
}

func toAnySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func toAnySliceOfMaps(values []map[string]any) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// ProjectRolePageURL opens the role's own editor rather than the project page,
// so the token can be generated without hunting through tabs.
func ProjectRolePageURL(webUIURL, project, role string) string {
	if webUIURL == "" {
		return ""
	}

	base := fmt.Sprintf("%s/settings/projects/%s", strings.TrimSuffix(webUIURL, "/"), url.PathEscape(project))
	if role == "" {
		return base
	}

	query := url.Values{"tab": {"roles"}, "editRole": {role}}
	return base + "?" + query.Encode()
}
