package install_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/agent/install"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewOf(t *testing.T, opts *install.InstallOptions) string {
	t.Helper()

	out := &bytes.Buffer{}
	opts.Out = out
	install.RenderReviewForTest(opts)
	return out.String()
}

// Everything the installer worked out has to appear here, because this screen is
// the only place anybody sees it.
func TestReview_ShowsWhatWasDetectedAsWellAsWhatWasChosen(t *testing.T) {
	review := reviewOf(t, completedTargetOptions(t))

	for _, expected := range []string{
		"Kubernetes context", "Cluster address", "Node architectures",
		"octopus-agent-production", "production",
		"Polling address", pollingAddress,
		"Environments", "Target tags", "Default namespace", "Tenanted deployments",
		"Storage class", "Access mode",
		"Chart", "Chart version", "Credentials", "Timeout",
	} {
		assert.Contains(t, review, expected)
	}
}

func TestReview_WorkerShowsPoolsRatherThanEnvironments(t *testing.T) {
	review := reviewOf(t, completedWorkerOptions(t))

	assert.Contains(t, review, "Worker pools")
	assert.Contains(t, review, "Kubernetes Pool")
	assert.NotContains(t, review, "Environments")
	assert.NotContains(t, review, "Target tags")
}

// The token is minted at install time, so the review can only promise what will
// be used - and must never show a credential.
func TestReview_NeverShowsACredential(t *testing.T) {
	opts := completedTargetOptions(t)
	review := reviewOf(t, opts)

	assert.Contains(t, review, "access token in a Kubernetes Secret")
	assert.NotContains(t, review, "eyJhbGciOiJIUzI1NiJ9")
}

func TestReview_ScriptPodPermissionsOnlyAppearWhenTheyAreAChoice(t *testing.T) {
	opts := completedTargetOptions(t)
	assert.NotContains(t, reviewOf(t, opts), "Permissions")

	opts.PermissionsController = true
	assert.Contains(t, reviewOf(t, opts), "the permissions controller can grant less than this per deployment")
}

// The review describes the fallback, which is all these values decide: a
// deployment that a WorkloadServiceAccount matches gets that instead.
func TestReview_DescribesEachKindOfScriptPodPermission(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.PermissionsController = true

	assert.Contains(t, reviewOf(t, opts), "anything in the cluster")

	opts.RestrictScriptPods.Value = true
	review := reviewOf(t, opts)
	assert.Contains(t, review, "nothing by default")
	assert.Contains(t, review, "the permissions controller grants each deployment what it needs")

	opts.RestrictScriptPods.Value = false
	opts.ScriptPodRoles.Value = []string{"deployer", "monitoring/reader"}
	opts.ScriptPodRules = []any{map[string]any{"verbs": []string{"get"}}}
	review = reviewOf(t, opts)
	assert.Contains(t, review, "the rules of deployer, monitoring/reader")
	assert.Contains(t, review, "1 rule copied in now, and not followed afterwards")
}

// The access mode is worked out rather than asked about, so the review has to
// say where it came from.
func TestReview_ReportsTheAccessModeAndWhereItCameFrom(t *testing.T) {
	opts := completedTargetOptions(t)
	review := reviewOf(t, opts)
	assert.Contains(t, review, "ReadWriteOnce - script pods run on the agent's node")
	assert.Contains(t, review, "kubernetes.io/gce-pd serves one node at a time")

	opts.StorageClass.Value = "filestore"
	opts.DeriveAccessModeForTest()
	review = reviewOf(t, opts)
	assert.Contains(t, review, "ReadWriteMany - script pods run on any node")
	assert.Contains(t, review, "filestore.csi.storage.gke.io serves a shared filesystem")

	opts.AccessModeChosen = true
	assert.Contains(t, reviewOf(t, opts), "(chosen)")
}

func TestConfirm_Cancelling(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("Ready to install?", "", []string{"Install", "Change a setting", "Cancel"}, "Cancel"),
	})

	opts := completedTargetOptions(t)
	opts.Ask = asker

	assert.ErrorContains(t, install.Confirm(context.Background(), opts), "cancelled")
	checkRemainingPrompts()
}

// Editing the name has to bring the namespace and release name with it, because
// both are derived from it.
func TestConfirm_EditingTheNameRederivesTheNamespace(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("Ready to install?", "", []string{"Install", "Change a setting", "Cancel"}, "Change a setting"),
		testutil.NewSelectPrompt("Which setting?", "", editableSettings(t), "Octopus: Name"),
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this deployment target.", "Staging"),
		testutil.NewSelectPrompt("Ready to install?", "", []string{"Install", "Change a setting", "Cancel"}, "Install"),
	})

	opts := completedTargetOptions(t)
	opts.Ask = asker

	require.NoError(t, install.Confirm(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, "Staging", opts.Name.Value)
	assert.Equal(t, "octopus-agent-staging", opts.TargetNamespace)
	assert.Equal(t, "staging", opts.TargetRelease)
}

// editableSettings is the "Which setting?" option list, which follows the review
// groups rather than being written out twice.
func editableSettings(t *testing.T) []string {
	t.Helper()

	return []string{
		"Cluster: Namespace",
		"Cluster: Helm release",
		"Octopus: Name",
		"Octopus: Polling address",
		"Octopus: Machine policy",
		"Octopus: Server certificate",
		"Deployment target: Environments",
		"Deployment target: Target tags",
		"Deployment target: Default namespace",
		"Deployment target: Tenanted deployments",
		"Script pods: Storage class",
		"Script pods: Access mode",
		"Helm: Chart version",
		"Helm: Credentials",
		"Helm: Timeout",
	}
}

// A name that is already a Helm release in this cluster upgrades that release
// rather than adding an agent, and one that is already the other kind of agent
// is the documented way to break both.
func TestReportFindings_ExistingReleaseOfTheOtherKind(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	require.NoError(t, opts.ResolveWithoutPrompting(context.Background()))
	opts.Installations = []agentK8s.Installation{{
		Release: helm.Release{Name: "production", Namespace: "octopus-agent-production", Chart: "kubernetes-agent", Version: "3.13.3"},
		Name:    "Production",
		Mode:    agentK8s.ModeWorker,
	}}

	out := &bytes.Buffer{}
	opts.Out = out
	install.ReportFindingsForTest(opts)

	assert.Contains(t, out.String(), "is already a worker")
	assert.Contains(t, out.String(), "different name")
}

func TestReportFindings_UpgradingAnExistingAgent(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	require.NoError(t, opts.ResolveWithoutPrompting(context.Background()))
	opts.Installations = []agentK8s.Installation{{
		Release: helm.Release{Name: "production", Namespace: "octopus-agent-production", Chart: "kubernetes-agent", Version: "3.13.3"},
		Name:    "Production",
		Mode:    agentK8s.ModeDeploymentTarget,
	}}

	out := &bytes.Buffer{}
	opts.Out = out
	install.ReportFindingsForTest(opts)

	assert.Contains(t, out.String(), "Upgrading the existing release")
}

func TestReportFindings_NameAlreadyRegisteredInOctopus(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	require.NoError(t, opts.ResolveWithoutPrompting(context.Background()))
	opts.RegisteredCallback = func(string) (bool, error) { return true, nil }

	out := &bytes.Buffer{}
	opts.Out = out
	install.ReportFindingsForTest(opts)

	assert.Contains(t, out.String(), "already has a deployment target named")
	assert.Contains(t, out.String(), "take that one over")
}

func TestReportFindings_UnsupportedNodeArchitecture(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	require.NoError(t, opts.ResolveWithoutPrompting(context.Background()))
	opts.NodeArchitectures = []string{"amd64", "ppc64le"}

	out := &bytes.Buffer{}
	opts.Out = out
	install.ReportFindingsForTest(opts)

	assert.Contains(t, out.String(), "ppc64le")
	assert.True(t, strings.Contains(out.String(), "cannot run on"))
}

// Choosing a tag Octopus has never seen creates it, which the review has to say
// rather than let it look like a match.
func TestReview_NamesTheTargetTagsThatWillBeCreated(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.KnownTargetTags = []string{"k8s", "web"}

	opts.Roles.Value = []string{"k8s"}
	assert.Contains(t, reviewOf(t, opts), "(chosen)")

	opts.Roles.Value = []string{"k8s", "k8s-agent"}
	review := reviewOf(t, opts)
	assert.Contains(t, review, "k8s, k8s-agent")
	assert.Contains(t, review, "k8s-agent created when the agent registers")
}
