package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/permissionscontroller/install"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const namespaceQuestion = "Which namespaces should the controller manage permissions in?"

const namespaceHelp = "Outside these namespaces the agent's own default script pod permissions apply, " +
	"and the controller does nothing."

var namespaceOptions = []string{"Every namespace", "Only namespaces I choose", "Namespaces matching a pattern"}

// clusterNamespaces is what the cluster reports, minus the kube-* namespaces the
// installer filters out.
var clusterNamespaces = []string{"default", "my-app", "octopus-agent-production"}

func agentInstallation(name, namespace string, mode agent.Mode, clusterRole bool) agent.Installation {
	return agent.Installation{
		Release:              helm.Release{Name: name, Namespace: namespace, Chart: agent.ChartName, Version: "2.30.0"},
		Name:                 name,
		Mode:                 mode,
		ScriptPodClusterRole: clusterRole,
	}
}

// newOptions starts from a cluster that already has cert-manager and one agent,
// which is what the prerequisites ask for and so asks the fewest questions.
func newOptions(t *testing.T, flags *install.InstallFlags, asker question.Asker) (*install.InstallOptions, *bytes.Buffer) {
	t.Helper()

	out := &bytes.Buffer{}
	opts := &install.InstallOptions{
		InstallFlags:       flags,
		Dependencies:       &cmd.Dependencies{Ask: asker, Out: out, CmdPath: "octopus kubernetes permissions-controller install"},
		CertManagerPresent: true,
		Agents:             []agent.Installation{agentInstallation("production", "octopus-agent-production", agent.ModeDeploymentTarget, true)},
		KubeContextInfo:    octoK8s.Context{Name: "colima-k8s", Server: "https://192.168.64.4:52409", IsCurrent: true},
	}
	opts.NamespacesCallback = func(context.Context) ([]string, error) { return clusterNamespaces, nil }
	return opts, out
}

func TestPromptMissing_NothingSupplied(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Every namespace"),
	})

	flags := install.NewInstallFlags()
	opts, _ := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Empty(t, flags.TargetNamespaces.Value, "every namespace is the chart's own default, so nothing is set")
	assert.Empty(t, flags.TargetNamespaceRegex.Value)
	assert.True(t, flags.CertManager.Value)
	assert.False(t, flags.NamespacedRBAC.Value)

	assert.Equal(t, octoK8s.PermissionsControllerNamespace, opts.TargetNamespace)
	assert.Equal(t, "octopus-permissions-controller", opts.TargetRelease)
}

func TestPromptMissing_ChoosingNamespacesOffersTheOnesInTheCluster(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Only namespaces I choose"),
		testutil.NewMultiSelectPrompt("Which namespaces?", "", clusterNamespaces, []string{"my-app", "default"}),
	})

	flags := install.NewInstallFlags()
	opts, _ := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, []string{"my-app", "default"}, flags.TargetNamespaces.Value)
}

func TestPromptMissing_PatternIsAskedForAsAnExpression(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Namespaces matching a pattern"),
		testutil.NewInputPrompt("Namespace pattern",
			"A regular expression matched against namespace names, which also covers namespaces that do not exist yet.",
			"^team-.*$"),
	})

	flags := install.NewInstallFlags()
	opts, _ := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, "^team-.*$", flags.TargetNamespaceRegex.Value)
	assert.Empty(t, flags.TargetNamespaces.Value)
}

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.TargetNamespaces.Value = []string{"my-app"}
	flags.NamespacedRBAC.Value = true

	opts, _ := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
}

// A namespace pattern is an answer to the same question, so being given one must
// not lead to being asked it again.
func TestPromptMissing_PatternFlagSuppressesTheNamespaceQuestion(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.TargetNamespaceRegex.Value = "^team-.*$"

	opts, _ := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
}

func TestPromptMissing_WithoutCertManagerOffersToCarryOn(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Install anyway, and supply the webhook certificate yourself?",
			"Answering no cancels the install, so you can install cert-manager first and start again.", true, false),
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Every namespace"),
	})

	flags := install.NewInstallFlags()
	opts, out := newOptions(t, flags, asker)
	opts.CertManagerPresent = false

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.False(t, flags.CertManager.Value, "carrying on means the chart must not be told to use cert-manager")
	assert.Contains(t, out.String(), "cert-manager is not installed in this cluster")
}

func TestPromptMissing_WithoutCertManagerCanBeCancelled(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Install anyway, and supply the webhook certificate yourself?",
			"Answering no cancels the install, so you can install cert-manager first and start again.", false, false),
	})

	flags := install.NewInstallFlags()
	opts, _ := newOptions(t, flags, asker)
	opts.CertManagerPresent = false

	assert.EqualError(t, install.PromptMissing(context.Background(), opts), "install cancelled")
}

func TestPromptMissing_ReportsWhatIsAlreadyInTheCluster(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Every namespace"),
	})

	flags := install.NewInstallFlags()
	opts, out := newOptions(t, flags, asker)
	opts.ExistingRelease = &helm.Release{
		Name: "octopus-permissions-controller", Namespace: "octopus-permissions-controller-system", Version: "1.2.0",
	}

	require.NoError(t, install.PromptMissing(context.Background(), opts))

	assert.Contains(t, out.String(), "already installed")
	assert.Contains(t, out.String(), "This install upgrades it")
	assert.Contains(t, out.String(), "production", "the agent it will act on is worth knowing about")
	assert.Contains(t, out.String(), "octopus-agent-production")
}

func TestPromptMissing_SaysWhenThereIsNoAgentToActOn(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(namespaceQuestion, namespaceHelp, namespaceOptions,
			"Every namespace", "Every namespace"),
	})

	flags := install.NewInstallFlags()
	opts, out := newOptions(t, flags, asker)
	opts.Agents = nil

	require.NoError(t, install.PromptMissing(context.Background(), opts))

	assert.Contains(t, out.String(), "No Kubernetes agent is installed in this cluster")
	assert.Contains(t, out.String(), "Installing it first is fine")
}

func TestBuildValues_Defaults(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, _ := newOptions(t, install.NewInstallFlags(), asker)

	values := opts.BuildValues()

	assert.Equal(t, map[string]any{
		"certManager": map[string]any{"enable": true},
		"rbac":        map[string]any{"namespaced": false},
	}, values)
	assert.NotContains(t, values, "manager", "neither namespace flag was set, so the chart's own env stays as it is")
}

func TestBuildValues_TargetNamespacesAreJoinedWithCommas(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.TargetNamespaces.Value = []string{"my-app", "other-app"}
	opts, _ := newOptions(t, flags, asker)

	values := opts.BuildValues()

	assert.Equal(t, map[string]any{
		"envOverrides": map[string]any{"TARGET_NAMESPACES": "my-app,other-app"},
	}, values["manager"])
}

func TestBuildValues_NamespaceRegex(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.TargetNamespaceRegex.Value = "^team-.*$"
	opts, _ := newOptions(t, flags, asker)

	values := opts.BuildValues()

	assert.Equal(t, map[string]any{
		"envOverrides": map[string]any{"TARGET_NAMESPACE_REGEX": "^team-.*$"},
	}, values["manager"])
}

func TestBuildValues_NamespacesAndRegexTogether(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.TargetNamespaces.Value = []string{"my-app"}
	flags.TargetNamespaceRegex.Value = "^team-.*$"
	opts, _ := newOptions(t, flags, asker)

	assert.Equal(t, map[string]any{
		"envOverrides": map[string]any{
			"TARGET_NAMESPACES":      "my-app",
			"TARGET_NAMESPACE_REGEX": "^team-.*$",
		},
	}, opts.BuildValues()["manager"])
}

func TestBuildValues_CertManagerOff(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.CertManager.Value = false
	opts, _ := newOptions(t, flags, asker)

	assert.Equal(t, map[string]any{"enable": false}, opts.BuildValues()["certManager"])
}

func TestBuildValues_NamespacedRBAC(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.NamespacedRBAC.Value = true
	opts, _ := newOptions(t, flags, asker)

	assert.Equal(t, map[string]any{"namespaced": true}, opts.BuildValues()["rbac"])
}

func TestResolveNames_FixedByDefault(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, _ := newOptions(t, install.NewInstallFlags(), asker)

	opts.ResolveNamesForTest()

	assert.Equal(t, "octopus-permissions-controller-system", opts.TargetNamespace)
	assert.Equal(t, "octopus-permissions-controller", opts.TargetRelease)
}

func TestResolveNames_FlagsOverride(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.Namespace.Value = "octopus"
	flags.ReleaseName.Value = "opc"
	opts, _ := newOptions(t, flags, asker)

	opts.ResolveNamesForTest()

	assert.Equal(t, "octopus", opts.TargetNamespace)
	assert.Equal(t, "opc", opts.TargetRelease)
}

// Only one controller runs per cluster, so an existing release has to be
// upgraded rather than joined by a second one.
func TestResolveNames_AdoptsAnExistingRelease(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, _ := newOptions(t, install.NewInstallFlags(), asker)
	opts.ExistingRelease = &helm.Release{Name: "opc", Namespace: "octopus", Version: "1.2.0"}

	opts.ResolveNamesForTest()

	assert.Equal(t, "octopus", opts.TargetNamespace)
	assert.Equal(t, "opc", opts.TargetRelease)
}

func TestConfirmPrerequisites_WithoutCertManagerAndNothingToAsk(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, out := newOptions(t, install.NewInstallFlags(), asker)
	opts.CertManagerPresent = false
	opts.NoPrompt = true

	err := opts.ConfirmPrerequisitesForTest()

	require.Error(t, err)
	assert.Contains(t, err.Error(), octoK8s.FlagSkipPreflight)
	assert.Contains(t, out.String(), "cert-manager")
	assert.Contains(t, out.String(), "--cert-manager=false", "the remediation has to name the way out of it")
}

// A dry run creates nothing, so an unmet prerequisite is worth reporting but not
// worth withholding the preview for.
func TestConfirmPrerequisites_WithoutCertManagerIsOnlyAWarningForADryRun(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.DryRun.Value = true
	opts, out := newOptions(t, flags, asker)
	opts.CertManagerPresent = false
	opts.NoPrompt = true

	require.NoError(t, opts.ConfirmPrerequisitesForTest())
	assert.Contains(t, out.String(), "cert-manager")
}

func TestConfirmPrerequisites_CertManagerOffNeedsNothingInstalled(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.CertManager.Value = false
	opts, _ := newOptions(t, flags, asker)
	opts.CertManagerPresent = false
	opts.NoPrompt = true

	require.NoError(t, opts.ConfirmPrerequisitesForTest())
}

func TestConfirmPrerequisites_CancellingAtTheWarning(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Continue with the install anyway?",
			"The controller is likely to install and then be unable to do its job.", false, false),
	})

	opts, _ := newOptions(t, install.NewInstallFlags(), asker)
	opts.CertManagerPresent = false

	assert.ErrorContains(t, opts.ConfirmPrerequisitesForTest(), "cancelled")
	checkRemainingPrompts()
}

func TestPrerequisiteChecks(t *testing.T) {
	tests := []struct {
		name               string
		certManagerPresent bool
		certManagerFlag    bool
		agents             []agent.Installation
		wantCertManager    octoK8s.CheckResult
		wantAgent          octoK8s.CheckResult
	}{
		{
			name:               "everything in place",
			certManagerPresent: true,
			certManagerFlag:    true,
			agents:             []agent.Installation{agentInstallation("production", "octopus-agent-production", agent.ModeWorker, true)},
			wantCertManager:    octoK8s.CheckPassed,
			wantAgent:          octoK8s.CheckPassed,
		},
		{
			name:            "cert-manager missing and wanted",
			certManagerFlag: true,
			wantCertManager: octoK8s.CheckFailed,
			wantAgent:       octoK8s.CheckSkipped,
		},
		{
			// Supplying the certificate another way is a supported choice, not a
			// failed prerequisite.
			name:            "cert-manager missing and not wanted",
			wantCertManager: octoK8s.CheckSkipped,
			wantAgent:       octoK8s.CheckSkipped,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

			flags := install.NewInstallFlags()
			flags.CertManager.Value = test.certManagerFlag
			opts, _ := newOptions(t, flags, asker)
			opts.CertManagerPresent = test.certManagerPresent
			opts.Agents = test.agents

			checks := opts.PrerequisiteChecks()
			require.Len(t, checks, 2)

			assert.Equal(t, "cert-manager", checks[0].Name)
			assert.Equal(t, test.wantCertManager, checks[0].Result)
			assert.Equal(t, "Kubernetes agents", checks[1].Name)
			assert.Equal(t, test.wantAgent, checks[1].Result)
		})
	}
}

// The controller takes nothing away on its own: until an agent stops granting
// its script pods cluster-wide permissions, an unmatched deployment carries on.
func TestPrintNextSteps_ShowsHowToRestrictEachAgentStillGrantingClusterWidePermissions(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, out := newOptions(t, install.NewInstallFlags(), asker)
	opts.Agents = []agent.Installation{
		agentInstallation("production", "octopus-agent-production", agent.ModeDeploymentTarget, true),
		agentInstallation("already-done", "octopus-agent-already-done", agent.ModeWorker, false),
	}

	opts.PrintNextSteps()
	printed := out.String()

	assert.Contains(t, printed, "apiVersion: agent.octopus.com/v1beta1")
	assert.Contains(t, printed, "kind: WorkloadServiceAccount")
	assert.Contains(t, printed,
		`--set scriptPods.serviceAccount.clusterRole.enabled="false" production `+
			"oci://registry-1.docker.io/octopusdeploy/kubernetes-agent")
	assert.Contains(t, printed, "--namespace octopus-agent-production")
	assert.NotContains(t, printed, "already-done", "an agent whose script pods are already restricted has nothing to do")
}

func TestPrintNextSteps_NoAgentsMeansNoHelmCommands(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts, out := newOptions(t, install.NewInstallFlags(), asker)
	opts.Agents = nil

	opts.PrintNextSteps()

	assert.Contains(t, out.String(), "kind: WorkloadServiceAccount")
	assert.NotContains(t, out.String(), "helm upgrade")
}

// The wizard builds these flags without cobra ever parsing a command line, so
// the two entry points have to agree that cert-manager is on by default.
func TestNewCmdInstall_CertManagerDefaultsOnInBothEntryPoints(t *testing.T) {
	assert.Equal(t, "true", install.NewCmdInstall(nil).Flags().Lookup(install.FlagCertManager).DefValue)
	assert.True(t, install.NewInstallFlags().CertManager.Value)
}

// GenerateAutomationCmd emits a bool flag only when it is true, so a
// --cert-manager that was turned off has to be spelled out to be reproducible.
func TestAutomationCmd_SpellsOutCertManagerWhenItIsTurnedOff(t *testing.T) {
	run := func(certManager bool) string {
		asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

		flags := install.NewInstallFlags()
		flags.CertManager.Value = certManager
		flags.TargetNamespaces.Value = []string{"my-app"}
		opts, out := newOptions(t, flags, asker)
		opts.ResolveNamesForTest()
		opts.ReportSuccessForTest(helm.Release{Name: "octopus-permissions-controller", Namespace: "ns", Version: "1.3.0"})
		return out.String()
	}

	assert.Contains(t, run(false), "--cert-manager=false")
	assert.NotContains(t, run(true), "--cert-manager")
	assert.Contains(t, run(true), "--target-namespace 'my-app'")
}
