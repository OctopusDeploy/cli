package argocd_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var appProjectGVR = schema.GroupVersionResource{
	Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects",
}

func appProject(name string, roles ...map[string]any) *unstructured.Unstructured {
	spec := map[string]any{}
	if len(roles) > 0 {
		items := make([]any, len(roles))
		for i, r := range roles {
			items[i] = r
		}
		spec["roles"] = items
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata":   map[string]any{"name": name, "namespace": "argocd"},
		"spec":       spec,
	}}
}

func role(name string, policies ...string) map[string]any {
	items := make([]any, len(policies))
	for i, p := range policies {
		items[i] = p
	}
	return map[string]any{"name": name, "policies": items}
}

func clusterWithProjects(objects ...runtime.Object) *octoK8s.Cluster {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{appProjectGVR: "AppProjectList"}

	return octoK8s.NewClusterForTesting(fake.NewSimpleClientset(), "test", "https://cluster").
		WithDynamic(dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...))
}

func octopusRole() argocd.ProjectRoleSpec {
	return argocd.ProjectRoleSpec{Project: "default", Role: "octopus", AllowSync: true}
}

func TestListProjects(t *testing.T) {
	c := clusterWithProjects(
		appProject("foo"),
		appProject("default", role("admin", "p, proj:default:admin, applications, *, default/*, allow")),
	)

	projects, err := argocd.ListProjects(context.Background(), c, "argocd")
	require.NoError(t, err)
	require.Len(t, projects, 2)

	assert.Equal(t, "default", projects[0].Name)
	assert.Equal(t, []string{"admin"}, projects[0].Roles)
	assert.Equal(t, "foo", projects[1].Name)
	assert.Contains(t, projects[0].Display(), "admin")
}

// A cluster where Argo CD is not managing anything has no CRDs, which is not a
// failure worth stopping the install for.
func TestListProjects_NoArgoCDCRDs(t *testing.T) {
	projects, err := argocd.ListProjects(context.Background(), clusterWithProjects(), "argocd")
	require.NoError(t, err)
	assert.Empty(t, projects)
}

// A project role token can only ever be granted things inside its project, so
// the policies are scoped and cluster resources are left out entirely.
func TestProjectRoleSpec_Policies(t *testing.T) {
	policies := octopusRole().Policies()

	assert.Contains(t, policies, "p, proj:default:octopus, applications, get, default/*, allow")
	assert.Contains(t, policies, "p, proj:default:octopus, applications, sync, default/*, allow")
	assert.Contains(t, policies, "p, proj:default:octopus, logs, get, default/*, allow")

	for _, p := range policies {
		assert.NotContains(t, p, "clusters", "a project role can never read cluster resources")
	}
}

func TestProjectRoleSpec_ReadOnlyOmitsSync(t *testing.T) {
	spec := octopusRole()
	spec.AllowSync = false

	for _, p := range spec.Policies() {
		assert.NotContains(t, p, "sync")
	}
}

func TestEnsureProjectRole_AddsTheRole(t *testing.T) {
	ctx := context.Background()
	c := clusterWithProjects(appProject("default", role("admin", "p, proj:default:admin, applications, *, default/*, allow")))

	status, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
	require.NoError(t, err)
	assert.False(t, status.Exists)

	require.NoError(t, argocd.EnsureProjectRole(ctx, c, "argocd", status))

	after, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
	require.NoError(t, err)
	assert.True(t, after.IsComplete())

	projects, err := argocd.ListProjects(ctx, c, "argocd")
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "octopus"}, projects[0].Roles, "the existing role must survive")
}

func TestEnsureProjectRole_TopsUpMissingPolicies(t *testing.T) {
	ctx := context.Background()
	c := clusterWithProjects(appProject("default",
		role("octopus", "p, proj:default:octopus, applications, get, default/*, allow")))

	status, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
	require.NoError(t, err)
	assert.True(t, status.Exists)
	assert.Len(t, status.MissingPolicies, 2)

	require.NoError(t, argocd.EnsureProjectRole(ctx, c, "argocd", status))

	after, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
	require.NoError(t, err)
	assert.True(t, after.IsComplete())
}

func TestEnsureProjectRole_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	c := clusterWithProjects(appProject("default"))

	for range 3 {
		status, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
		require.NoError(t, err)
		require.NoError(t, argocd.EnsureProjectRole(ctx, c, "argocd", status))
	}

	status, err := argocd.InspectProjectRole(ctx, c, "argocd", octopusRole())
	require.NoError(t, err)
	assert.Len(t, status.Spec.Policies(), 3)
	assert.True(t, status.IsComplete())
}

func TestProjectRolePatchPlan_ShowsOnlyWhatIsMissing(t *testing.T) {
	ctx := context.Background()
	c := clusterWithProjects(
		appProject("default"),
		appProject("done", role("octopus", octopusRoleFor("done")...)),
	)

	var statuses []argocd.ProjectRoleStatus
	for _, name := range []string{"default", "done"} {
		spec := argocd.ProjectRoleSpec{Project: name, Role: "octopus", AllowSync: true}
		status, err := argocd.InspectProjectRole(ctx, c, "argocd", spec)
		require.NoError(t, err)
		statuses = append(statuses, status)
	}

	plan := argocd.ProjectRolePatchPlan("argocd", statuses)
	assert.Contains(t, plan, "appproject/default")
	assert.NotContains(t, plan, "appproject/done", "a project that already grants everything needs no change")
}

func octopusRoleFor(project string) []string {
	return argocd.ProjectRoleSpec{Project: project, Role: "octopus", AllowSync: true}.Policies()
}

// Linking at the project alone leaves someone hunting through tabs for the
// role that actually needs a token.
func TestProjectRolePageURL(t *testing.T) {
	url := argocd.ProjectRolePageURL("https://argo.example.com", "default", "admin")
	assert.Equal(t, "https://argo.example.com/settings/projects/default?editRole=admin&tab=roles", url)

	assert.Equal(t, "https://argo.example.com/settings/projects/default",
		argocd.ProjectRolePageURL("https://argo.example.com/", "default", ""))
	assert.Empty(t, argocd.ProjectRolePageURL("", "default", "admin"))
}
