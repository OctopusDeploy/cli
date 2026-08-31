package install_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/permissionscontroller/install"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
)

func reviewOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.KubeContext.Value = "colima-k8s"

	return &install.InstallOptions{
		InstallFlags:       flags,
		Dependencies:       &cmd.Dependencies{Ask: asker, Out: &bytes.Buffer{}},
		CertManagerPresent: true,
		Agents: []agent.Installation{
			agentInstallation("production", "octopus-agent-production", agent.ModeDeploymentTarget, true),
		},
		KubeContextInfo: octoK8s.Context{Name: "colima-k8s", Server: "https://192.168.64.4:52409", IsCurrent: true},
	}
}

func reviewOf(t *testing.T, opts *install.InstallOptions) string {
	t.Helper()

	out := &bytes.Buffer{}
	opts.Out = out
	install.RenderReviewForTest(opts)
	return out.String()
}

// Almost everything here is worked out rather than asked for, so the review is
// the only place a person sees it before anything is created.
func TestReview_ShowsEverySetting(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	for _, expected := range []string{
		"colima-k8s",                            // chosen cluster
		"https://192.168.64.4:52409",            // detected cluster address
		"octopus-permissions-controller-system", // fixed namespace
		"octopus-permissions-controller",        // fixed release name
		"every namespace",                       // the chart's own default
		"cluster-wide",                          // RBAC scope
		"cert-manager",                          // who issues the webhook certificate
		install.ChartRef.Ref,
		octoK8s.DefaultTimeout.String(),
		"(one controller per cluster)", // why the names are what they are
	} {
		assert.Contains(t, review, expected)
	}
}

// The controller only ever acts on the script pods an agent creates, so what is
// already installed decides whether this install changes anything.
func TestReview_ListsTheAgentsFound(t *testing.T) {
	opts := reviewOptions(t)
	opts.Agents = []agent.Installation{
		agentInstallation("production", "octopus-agent-production", agent.ModeDeploymentTarget, true),
		agentInstallation("build-workers", "octopus-agent-build-workers", agent.ModeWorker, false),
	}

	review := reviewOf(t, opts)

	assert.Contains(t, review, "production")
	assert.Contains(t, review, "deployment target in octopus-agent-production")
	assert.Contains(t, review, "script pods still hold cluster-wide permissions")
	assert.Contains(t, review, "build-workers")
	assert.Contains(t, review, "worker in octopus-agent-build-workers")
	assert.Contains(t, review, "script pod permissions already restricted")
}

func TestReview_SaysWhenNoAgentIsInstalled(t *testing.T) {
	opts := reviewOptions(t)
	opts.Agents = nil

	review := reviewOf(t, opts)

	assert.Contains(t, review, "none installed in this cluster")
}

func TestReview_ShowsTheChosenNamespaces(t *testing.T) {
	opts := reviewOptions(t)
	opts.TargetNamespaces.Value = []string{"my-app", "other-app"}
	opts.TargetNamespaceRegex.Value = "^team-.*$"

	review := reviewOf(t, opts)

	assert.Contains(t, review, "my-app, other-app")
	assert.Contains(t, review, "^team-.*$")
	assert.NotContains(t, review, "every namespace")
}

func TestReview_ShowsWhenTheCertificateIsNotComingFromCertManager(t *testing.T) {
	opts := reviewOptions(t)
	opts.CertManagerPresent = false
	opts.CertManager.Value = false

	review := reviewOf(t, opts)

	assert.Contains(t, review, "supplied by you")
	assert.Contains(t, review, "cert-manager is not installed")
}

func TestReview_ShowsThatAnExistingControllerWillBeUpgraded(t *testing.T) {
	opts := reviewOptions(t)
	opts.ExistingRelease = &helm.Release{Name: "opc", Namespace: "octopus", Version: "1.2.0"}

	review := reviewOf(t, opts)

	assert.Contains(t, review, "Already installed")
	assert.Contains(t, review, "1.2.0")
	assert.Contains(t, review, "this install upgrades it")
	assert.Contains(t, review, "opc", "the existing release is what gets upgraded")
}

// Claiming a source for something that was never found reads as a detection that
// succeeded.
func TestReview_ClaimsNoSourceForAnUnsetValue(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	for _, line := range strings.Split(review, "\n") {
		if strings.Contains(line, "Namespace pattern") {
			assert.Contains(t, line, "(not set)")
			assert.NotContains(t, line, "chosen")
		}
	}
}

// --namespaced-rbac is not asked about, so the review is the only place its
// effect is described rather than named.
func TestReview_NamespacedRBACIsDescribedRatherThanNamed(t *testing.T) {
	opts := reviewOptions(t)
	opts.NamespacedRBAC.Value = true

	assert.Contains(t, rbacScopeLine(t, reviewOf(t, opts)), "this namespace only")
	assert.Contains(t, rbacScopeLine(t, reviewOf(t, reviewOptions(t))), "cluster-wide")
}

func rbacScopeLine(t *testing.T, review string) string {
	t.Helper()

	for _, line := range strings.Split(review, "\n") {
		if strings.Contains(line, "RBAC scope") {
			return line
		}
	}
	t.Fatal("the review did not show the RBAC scope")
	return ""
}
