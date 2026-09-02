package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/accesstokens"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/agent/install"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const octopusHost = "https://my.octopus.app"

// pollingAddress is what DerivePollingURL makes of octopusHost: Octopus Cloud
// serves polling Tentacles on their own hostname.
const pollingAddress = "https://polling.my.octopus.app"

func storageClasses() []octoK8s.StorageClass {
	return []octoK8s.StorageClass{
		{Name: "standard", Provisioner: "kubernetes.io/gce-pd", IsDefault: true},
		{Name: "filestore", Provisioner: "filestore.csi.storage.gke.io"},
	}
}

// clusterRoles are what an agent's script pods could be given the rules of. The
// system: roles Kubernetes ships are left out of the list the installer offers.
func clusterRoles() []runtime.Object {
	return []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "deployer"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list", "create", "update"}},
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-admin"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "system:discovery"},
			Rules:      []rbacv1.PolicyRule{{NonResourceURLs: []string{"/healthz"}, Verbs: []string{"get"}}},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "reader", Namespace: "monitoring"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-signer", Namespace: "kube-system"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}},
		},
	}
}

func newOptions(t *testing.T, flags *install.InstallFlags, mode agentK8s.Mode, asker func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error) *install.InstallOptions {
	t.Helper()

	dependencies := &cmd.Dependencies{
		Ask:   asker,
		Out:   &bytes.Buffer{},
		Host:  octopusHost,
		Space: &spaces.Space{Name: "Default"},
	}
	dependencies.Space.ID = "Spaces-1"

	opts := &install.InstallOptions{
		InstallFlags: flags,
		Dependencies: dependencies,
		Mode:         mode,

		CreateTargetEnvironmentOptions: &sharedTarget.CreateTargetEnvironmentOptions{
			Dependencies: dependencies,
			GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
				return []*environments.Environment{environments.NewEnvironment("Development"), environments.NewEnvironment("Production")}, nil
			},
		},
		CreateTargetMachinePolicyOptions: &machinescommon.CreateTargetMachinePolicyOptions{
			Dependencies: dependencies,
			GetAllMachinePoliciesCallback: func() ([]*machines.MachinePolicy, error) {
				return []*machines.MachinePolicy{machines.NewMachinePolicy("Default Machine Policy")}, nil
			},
		},
		WorkerPoolOptions: &sharedWorker.WorkerPoolOptions{
			Dependencies: dependencies,
			GetAllWorkerPoolsCallback: func() ([]*workerpools.WorkerPoolListResult, error) {
				return []*workerpools.WorkerPoolListResult{
					{Name: "Default Worker Pool", CanAddWorkers: true},
					{Name: "Kubernetes Pool", CanAddWorkers: true},
				}, nil
			},
		},

		AccessTokenCallback: func() (accesstokens.Token, error) {
			return accesstokens.Token{Value: "eyJhbGciOiJIUzI1NiJ9.token"}, nil
		},
		RegisteredCallback: func(string) (bool, error) { return false, nil },
		TargetTagsCallback: func() ([]string, error) { return []string{"k8s", "web"}, nil },

		Cluster:           octoK8s.NewClusterForTesting(fake.NewSimpleClientset(clusterRoles()...), "test", "https://cluster"),
		StorageClasses:    storageClasses(),
		NodeArchitectures: []string{"amd64"},
	}
	return opts
}

func TestPromptMissing_DeploymentTargetWithNothingSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you accept it?", "", true, true),
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this deployment target.", "Production"),
		testutil.NewMultiSelectPrompt("Choose at least one environment for the deployment target.\n", "",
			[]string{"Development", "Production"}, []string{"Production"}),
		testutil.NewMultiSelectWithAddPrompt("Which target tags should this deployment target have?\n", "",
			[]string{"k8s", "web"}, []string{"k8s"}, "tag"),
		testutil.NewInputPrompt("Default namespace for deployments (optional)",
			"Used only when neither the step nor the manifest names a namespace. "+
				"Leave it blank to make every step say where it deploys to.", ""),
		testutil.NewInputPromptWithDefault("Octopus Server polling address",
			"The agent polls Octopus over TCP, on port 10943 by default, separately from the REST API on 443. "+
				"Octopus Cloud serves this on its own hostname over 443. The connection has to reach Octopus intact - SSL offloading does not work.",
			pollingAddress, pollingAddress),
		testutil.NewSelectPrompt("Which storage class should the agent use?", "",
			[]string{
				"Use the cluster's default storage class",
				"standard (cluster default, kubernetes.io/gce-pd)",
				"filestore (filestore.csi.storage.gke.io)",
			}, "Use the cluster's default storage class"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := install.NewInstallFlags()
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, "Production", flags.Name.Value)
	assert.Equal(t, []string{"Production"}, flags.Environments.Value)
	assert.Equal(t, []string{"k8s"}, flags.Roles.Value)
	assert.Empty(t, flags.TenantedDeploymentMode.Value, "tenanted deployments are not asked about, and default to untenanted")
	assert.Equal(t, []string{pollingAddress}, flags.ServerCommsAddresses.Value)
	assert.True(t, flags.AcceptEula.Value)

	// Derived rather than asked for.
	assert.Equal(t, "octopus-agent-production", opts.TargetNamespace)
	assert.Equal(t, "production", opts.TargetRelease)
	assert.Empty(t, flags.StorageClass.Value, "the cluster default leaves the chart's own default in place")
}

// Whether script pods can spread across nodes follows from the storage class,
// so it is worked out rather than asked about.
func TestPromptMissing_AccessModeFollowsTheStorageClass(t *testing.T) {
	tests := []struct {
		name          string
		answer        string
		storageClass  string
		readWriteMany bool
	}{
		{"a shared filesystem", "filestore (filestore.csi.storage.gke.io)", "filestore", true},
		{"a block device", "standard (cluster default, kubernetes.io/gce-pd)", "standard", false},
		{"the cluster default is read the same way", "Use the cluster's default storage class", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
				testutil.NewSelectPrompt("Which storage class should the agent use?", "",
					[]string{
						"Use the cluster's default storage class",
						"standard (cluster default, kubernetes.io/gce-pd)",
						"filestore (filestore.csi.storage.gke.io)",
					}, tt.answer),
			})

			flags := allSuppliedTargetFlags()
			flags.StorageClass.Value = ""
			opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

			require.NoError(t, install.PromptMissing(context.Background(), opts))
			checkRemainingPrompts()

			assert.Equal(t, tt.storageClass, flags.StorageClass.Value)
			assert.Equal(t, tt.readWriteMany, flags.ReadWriteMany.Value)
		})
	}
}

// A shared filesystem is only worth having if the volume can be mounted from
// more than one node, so --read-write-many still wins where it is given.
func TestResolveWithoutPrompting_ReadWriteManyOverridesTheStorageClass(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.StorageClass.Value = "standard"
	flags.ReadWriteMany.Value = true
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.AccessModeChosen = true

	out := &bytes.Buffer{}
	opts.Out = out

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.True(t, flags.ReadWriteMany.Value)
	assert.Contains(t, out.String(), "not known to serve one", "a class that cannot do it is worth a warning")
}

func TestResolveWithoutPrompting_ReadWriteManyOnASharedFilesystemIsNotWarnedAbout(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.StorageClass.Value = "filestore"
	flags.ReadWriteMany.Value = true
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.AccessModeChosen = true

	out := &bytes.Buffer{}
	opts.Out = out

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.NotContains(t, out.String(), "not known to serve one")
}

func TestPromptMissing_NoStorageClassesAsksNothing(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.StorageClass.Value = ""
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.StorageClasses = nil

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
	assert.Empty(t, flags.StorageClass.Value)
}

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
}

func TestPromptMissing_WorkerAsksAboutPoolsInsteadOfEnvironments(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you accept it?", "", true, true),
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this worker.", "Cluster Worker"),
		testutil.NewMultiSelectPrompt("Select the worker pools to assign to the worker", "",
			[]string{"Default Worker Pool", "Kubernetes Pool"}, []string{"Kubernetes Pool"}),
		testutil.NewInputPromptWithDefault("Octopus Server polling address",
			"The agent polls Octopus over TCP, on port 10943 by default, separately from the REST API on 443. "+
				"Octopus Cloud serves this on its own hostname over 443. The connection has to reach Octopus intact - SSL offloading does not work.",
			pollingAddress, pollingAddress),
		testutil.NewSelectPrompt("Which storage class should the agent use?", "",
			[]string{
				"Use the cluster's default storage class",
				"standard (cluster default, kubernetes.io/gce-pd)",
				"filestore (filestore.csi.storage.gke.io)",
			}, "Use the cluster's default storage class"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := install.NewInstallFlags()
	opts := newOptions(t, flags, agentK8s.ModeWorker, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, []string{"Kubernetes Pool"}, flags.WorkerPools.Value)
	assert.Empty(t, flags.Environments.Value, "a worker has no environments")
	assert.Equal(t, "octopus-worker-cluster-worker", opts.TargetNamespace,
		"a worker lands in the same namespace a portal install of the same name would")
}

const permissionsQuestion = "What permissions should be used by workloads out of any WSA scope?"

const permissionsHelp = "A workload that no WorkloadServiceAccount matches falls back to these. Granting nothing means such a " +
	"workload fails rather than running with more access than it should have; anything is the chart's own " +
	"default of the whole cluster."

var permissionsOptions = []string{"Nothing", "Anything", "Copy an existing role"}

// The permissions controller is the only thing that can grant a script pod
// permissions once the chart's own default is taken away, so the question is
// only worth asking where it is installed.
func TestPromptMissing_ScriptPodPermissionsAreNotAskedAboutWithoutTheController(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
	assert.False(t, opts.RestrictScriptPods.Value)
}

func TestPromptMissing_ScriptPodPermissionsGrantNothing(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(permissionsQuestion, permissionsHelp, permissionsOptions, "Nothing", "Nothing"),
	})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	opts.PermissionsController = true

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
	assert.True(t, opts.RestrictScriptPods.Value)
	assert.Empty(t, opts.ScriptPodRules)
}

func TestPromptMissing_ScriptPodPermissionsKeepTheChartDefault(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(permissionsQuestion, permissionsHelp, permissionsOptions, "Nothing", "Anything"),
	})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	opts.PermissionsController = true

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
	assert.False(t, opts.RestrictScriptPods.Value)
	assert.Empty(t, opts.ScriptPodRules)
}

// The rules are copied rather than referenced, because that is what the chart
// takes. The roles Kubernetes ships are not worth offering.
func TestPromptMissing_ScriptPodPermissionsCopiedFromAClusterRole(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPromptWithDefault(permissionsQuestion, permissionsHelp, permissionsOptions, "Nothing", "Copy an existing role"),
		testutil.NewMultiSelectPrompt("Which roles should they be given the rules of?", "",
			[]string{
				"cluster-admin (cluster role, full access to the cluster)",
				"deployer (cluster role, 1 rule)",
				"monitoring/reader (role, 1 rule)",
			},
			[]string{"deployer (cluster role, 1 rule)", "monitoring/reader (role, 1 rule)"}),
	})

	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	opts.PermissionsController = true

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.False(t, opts.RestrictScriptPods.Value)
	assert.Equal(t, []string{"deployer", "monitoring/reader"}, opts.ScriptPodRoles.Value)
	assert.Equal(t, []any{
		map[string]any{
			"apiGroups": []string{"apps"},
			"resources": []string{"deployments"},
			"verbs":     []string{"get", "list", "create", "update"},
		},
		map[string]any{
			"apiGroups": []string{""},
			"resources": []string{"pods"},
			"verbs":     []string{"get"},
		},
	}, opts.ScriptPodRules, "the rules of every chosen role are gathered into one list")
}

func TestResolveScriptPodRole_CopiesTheRulesByName(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ScriptPodRoles.Value = []string{"deployer"}
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.Len(t, opts.ScriptPodRules, 1)
}

func TestResolveScriptPodRole_RejectsAnUnknownRole(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ScriptPodRoles.Value = []string{"does-not-exist"}
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	assert.ErrorContains(t, opts.ResolveWithoutPrompting(), "does-not-exist")
}

// Granting nothing and granting a role's rules are opposite answers to the same
// question, so asking for both is a mistake worth naming.
func TestResolveScriptPodRole_RejectsBothWaysAtOnce(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ScriptPodRoles.Value = []string{"deployer"}
	flags.RestrictScriptPods.Value = true
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	err := opts.ResolveWithoutPrompting()
	require.Error(t, err)
	assert.Contains(t, err.Error(), install.FlagRestrictScriptPods)
	assert.Contains(t, err.Error(), install.FlagScriptPodRole)
}

func TestPromptMissing_DecliningTheAgreementStops(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you accept it?", "", false, true),
	})

	opts := newOptions(t, install.NewInstallFlags(), agentK8s.ModeDeploymentTarget, asker)

	err := install.PromptMissing(context.Background(), opts)
	assert.ErrorContains(t, err, "Customer Agreement")
}

func TestPromptMissing_DerivesNamespaceFromName(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.Name.Value = "EU West Production"
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	assert.Equal(t, "octopus-agent-eu-west-production", opts.TargetNamespace)
	assert.Equal(t, "eu-west-production", opts.TargetRelease)
}

func allSuppliedTargetFlags() *install.InstallFlags {
	flags := install.NewInstallFlags()
	flags.Name.Value = "Production"
	flags.Environments.Value = []string{"Production"}
	flags.Roles.Value = []string{"k8s"}
	flags.TenantedDeploymentMode.Value = sharedTarget.Untenanted
	flags.DefaultNamespace.Value = "production"
	flags.ServerCommsAddresses.Value = []string{pollingAddress}
	flags.StorageClass.Value = "standard"
	flags.AcceptEula.Value = true
	return flags
}

func allSuppliedWorkerFlags() *install.InstallFlags {
	flags := install.NewInstallFlags()
	flags.Name.Value = "Cluster Worker"
	flags.WorkerPools.Value = []string{"Kubernetes Pool"}
	flags.ServerCommsAddresses.Value = []string{pollingAddress}
	flags.AcceptEula.Value = true
	return flags
}

func completedTargetOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts := newOptions(t, allSuppliedTargetFlags(), agentK8s.ModeDeploymentTarget, asker)
	require.NoError(t, opts.ResolveWithoutPrompting())
	return opts
}

func completedWorkerOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	opts := newOptions(t, allSuppliedWorkerFlags(), agentK8s.ModeWorker, asker)
	require.NoError(t, opts.ResolveWithoutPrompting())
	return opts
}

func TestBuildValues_DeploymentTargetRegistration(t *testing.T) {
	opts := completedTargetOptions(t)

	values, err := opts.BuildValues()
	require.NoError(t, err)

	agentValues := values["agent"].(map[string]any)
	assert.Equal(t, "Production", agentValues["name"])
	assert.Equal(t, "Y", agentValues["acceptEula"])
	assert.Equal(t, octopusHost, agentValues["serverUrl"])
	assert.Equal(t, []string{pollingAddress}, agentValues["serverCommsAddresses"])
	assert.Equal(t, "Default", agentValues["space"])
	assert.NotContains(t, agentValues, "worker", "a deployment target does not register as a worker")

	target := agentValues["deploymentTarget"].(map[string]any)
	assert.Equal(t, true, target["enabled"])

	initial := target["initial"].(map[string]any)
	assert.Equal(t, []string{"Production"}, initial["environments"])
	assert.Equal(t, []string{"k8s"}, initial["tags"])
	assert.Equal(t, "Untenanted", initial["tenantedDeploymentParticipation"])
	assert.Equal(t, "production", initial["defaultNamespace"])
	assert.NotContains(t, initial, "tenants")
	assert.NotContains(t, initial, "tenantTags")
}

func TestBuildValues_WorkerRegistration(t *testing.T) {
	opts := completedWorkerOptions(t)

	values, err := opts.BuildValues()
	require.NoError(t, err)

	agentValues := values["agent"].(map[string]any)
	assert.NotContains(t, agentValues, "deploymentTarget", "a worker does not register as a deployment target")

	worker := agentValues["worker"].(map[string]any)
	assert.Equal(t, true, worker["enabled"])
	assert.Equal(t, []string{"Kubernetes Pool"}, worker["initial"].(map[string]any)["workerPools"])
}

func TestBuildValues_TenantsAreOnlySentWhenChosen(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.TenantedDeploymentMode.Value = sharedTarget.TenantedOrUntenanted
	opts.Tenants.Value = []string{"Valley Veterinary Clinic"}
	opts.TenantTags.Value = []string{"Importance/VIP"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	initial := values["agent"].(map[string]any)["deploymentTarget"].(map[string]any)["initial"].(map[string]any)
	assert.Equal(t, "TenantedOrUntenanted", initial["tenantedDeploymentParticipation"])
	assert.Equal(t, []string{"Valley Veterinary Clinic"}, initial["tenants"])
	assert.Equal(t, []string{"Importance/VIP"}, initial["tenantTags"])
}

func TestBuildValues_AccessTokenGoesIntoASecretByDefault(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.Token = accesstokens.Token{Value: "eyJhbGciOiJIUzI1NiJ9.token"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	agentValues := values["agent"].(map[string]any)
	assert.Equal(t, "octopus-agent-registration-token", agentValues["bearerTokenSecretName"])
	assert.NotContains(t, agentValues, "bearerToken", "the access token must not reach the Helm values")
	for _, key := range []string{"serverApiKey", "serverApiKeySecretName", "username", "password"} {
		assert.NotContains(t, agentValues, key, "no long-lived credential belongs in the cluster")
	}
}

func TestBuildValues_InlineSecretsOptsIn(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.InlineSecrets.Value = true
	opts.Token = accesstokens.Token{Value: "eyJhbGciOiJIUzI1NiJ9.token"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	agentValues := values["agent"].(map[string]any)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.token", agentValues["bearerToken"])
	assert.NotContains(t, agentValues, "bearerTokenSecretName")
}

// A dry run never asks Octopus for a token, so the values it renders have to
// reference the Secret a real install would write.
func TestBuildValues_DryRunWithInlineSecretsStillReferencesTheSecret(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.InlineSecrets.Value = true

	values, err := opts.BuildValues()
	require.NoError(t, err)

	agentValues := values["agent"].(map[string]any)
	assert.Equal(t, "octopus-agent-registration-token", agentValues["bearerTokenSecretName"])
	assert.NotContains(t, agentValues, "bearerToken")
}

func TestBuildValues_StorageFollowsWhatWasChosen(t *testing.T) {
	tests := []struct {
		name          string
		storageClass  string
		readWriteMany bool
		expected      map[string]any
	}{
		{"the cluster default leaves persistence alone", "", false, nil},
		{"a storage class on its own", "filestore", false, map[string]any{"storageClassName": "filestore"}},
		{"ReadWriteMany for script pods on any node", "filestore", true, map[string]any{
			"storageClassName": "filestore",
			"accessModes":      []string{"ReadWriteMany"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := completedTargetOptions(t)
			opts.StorageClass.Value = tt.storageClass
			opts.ReadWriteMany.Value = tt.readWriteMany

			values, err := opts.BuildValues()
			require.NoError(t, err)

			if tt.expected == nil {
				assert.NotContains(t, values, "persistence")
				return
			}
			assert.Equal(t, tt.expected, values["persistence"])
		})
	}
}

func TestBuildValues_RestrictedScriptPodsGiveUpTheClusterRole(t *testing.T) {
	opts := completedTargetOptions(t)
	assert.NotContains(t, mustBuild(t, opts), "scriptPods")

	opts.RestrictScriptPods.Value = true
	scriptPods := mustBuild(t, opts)["scriptPods"].(map[string]any)
	assert.Equal(t, false,
		scriptPods["serviceAccount"].(map[string]any)["clusterRole"].(map[string]any)["enabled"])
}

func TestBuildValues_OptionalAgentSettings(t *testing.T) {
	opts := completedTargetOptions(t)
	agentValues := mustBuild(t, opts)["agent"].(map[string]any)
	assert.NotContains(t, agentValues, "machinePolicyName")
	assert.NotContains(t, agentValues, "serverCertificate")

	opts.MachinePolicy.Value = "Kubernetes Machine Policy"
	opts.ServerCertificate.Value = "LS0tLS1CRUdJTg=="
	agentValues = mustBuild(t, opts)["agent"].(map[string]any)
	assert.Equal(t, "Kubernetes Machine Policy", agentValues["machinePolicyName"])
	assert.Equal(t, "LS0tLS1CRUdJTg==", agentValues["serverCertificate"])
}

func TestBuildValues_NeedsAPollingAddress(t *testing.T) {
	opts := completedTargetOptions(t)
	opts.ServerCommsAddresses.Value = nil

	_, err := opts.BuildValues()
	assert.ErrorContains(t, err, install.FlagServerCommsAddress)
}

func mustBuild(t *testing.T, opts *install.InstallOptions) map[string]any {
	t.Helper()

	values, err := opts.BuildValues()
	require.NoError(t, err)
	return values
}

func TestValidateForAutomation_DeploymentTarget(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*install.InstallFlags)
		missing []string
	}{
		{"nothing supplied", func(*install.InstallFlags) {}, []string{"--name", "--environment", "--role or --tag", "--accept-eula"}},
		{"no environment", func(f *install.InstallFlags) {
			f.Name.Value = "Production"
			f.Roles.Value = []string{"k8s"}
			f.AcceptEula.Value = true
		}, []string{"--environment"}},
		{"no role or tag", func(f *install.InstallFlags) {
			f.Name.Value = "Production"
			f.Environments.Value = []string{"Production"}
			f.AcceptEula.Value = true
		}, []string{"--role or --tag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
			flags := install.NewInstallFlags()
			tt.prepare(flags)

			opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
			opts.NoPrompt = true

			err := opts.ValidateForAutomation()
			require.Error(t, err)
			for _, expected := range tt.missing {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}

func TestValidateForAutomation_WorkerNeedsAPool(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.Name.Value = "Cluster Worker"
	flags.AcceptEula.Value = true

	opts := newOptions(t, flags, agentK8s.ModeWorker, asker)
	opts.NoPrompt = true

	err := opts.ValidateForAutomation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--worker-pool")
	assert.NotContains(t, err.Error(), "--environment", "a worker has no environments")
}

// A dry run registers nothing, so there is no agreement being entered into.
func TestValidateForAutomation_DryRunNeedsNoAgreement(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.AcceptEula.Value = false
	flags.DryRun.Value = true

	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.NoPrompt = true

	require.NoError(t, opts.ValidateForAutomation())
}

func TestPromptMissing_RejectsAnUnknownMachinePolicy(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.MachinePolicy.Value = "Does Not Exist"
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	assert.ErrorContains(t, install.PromptMissing(context.Background(), opts), "Does Not Exist")
}

func TestResolveWithoutPrompting_RejectsAnUnknownMachinePolicy(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.MachinePolicy.Value = "Does Not Exist"
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	assert.ErrorContains(t, opts.ResolveWithoutPrompting(), "Does Not Exist")
}

func TestResolveWithoutPrompting_DerivesThePollingAddress(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ServerCommsAddresses.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.Equal(t, []string{pollingAddress}, flags.ServerCommsAddresses.Value)
}

// A space can easily have only dynamic pools, which Octopus runs on its own
// machines, so an empty list is a likely dead end rather than a rare one.
func TestPromptMissing_NoPoolCanHoldAWorker(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you accept it?", "", true, true),
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this worker.", "Cluster Worker"),
	})

	opts := newOptions(t, install.NewInstallFlags(), agentK8s.ModeWorker, asker)
	opts.GetAllWorkerPoolsCallback = func() ([]*workerpools.WorkerPoolListResult, error) { return nil, nil }

	err := install.PromptMissing(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no worker pool in space Default can hold a worker")
	assert.Contains(t, err.Error(), "worker-pool static create")
	checkRemainingPrompts()
}

func TestValidateWorkerPools_RejectsAPoolThatCannotHoldAWorker(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedWorkerFlags()
	flags.WorkerPools.Value = []string{"Kubernetes Pool", "Hosted Pool"}
	opts := newOptions(t, flags, agentK8s.ModeWorker, asker)

	err := opts.ResolveWithoutPrompting()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Hosted Pool")
	assert.NotContains(t, err.Error(), "Kubernetes Pool", "only the pools that could not be found are named")
}

// A deployment target has no worker pools, so nothing supplied there should be
// checked against them.
func TestValidateWorkerPools_IgnoredForADeploymentTarget(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.WorkerPools.Value = []string{"Hosted Pool"}
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, opts.ResolveWithoutPrompting())
}

// Octopus creates a target tag as soon as a target registers with it, so a tag
// that is not in the list is a normal answer rather than a mistake.
func TestPromptMissing_ATargetTagCanBeNew(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewMultiSelectWithAddPrompt("Which target tags should this deployment target have?\n", "",
			[]string{"k8s", "web"}, []string{"k8s", "k8s-agent"}, "tag"),
	})

	flags := allSuppliedTargetFlags()
	flags.Roles.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, []string{"k8s", "k8s-agent"}, flags.Roles.Value)
	assert.Equal(t, []string{"k8s-agent"}, opts.NewTargetTagsForTest(), "only the tag Octopus has never seen is new")
}

// The tags come from every target tag set at once. Which set a tag belongs to
// is not something to make somebody answer while installing an agent.
func TestPromptMissing_TargetTagsAreAskedForOnce(t *testing.T) {
	asked := 0
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewMultiSelectWithAddPrompt("Which target tags should this deployment target have?\n", "",
			[]string{"cloud-aws", "cloud-gcp", "k8s"}, []string{"k8s", "cloud-gcp"}, "tag"),
	})

	flags := allSuppliedTargetFlags()
	flags.Roles.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.TargetTagsCallback = func() ([]string, error) {
		asked++
		return []string{"cloud-aws", "cloud-gcp", "k8s"}, nil
	}

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, []string{"k8s", "cloud-gcp"}, flags.Roles.Value)
	assert.Equal(t, 1, asked, "the space's tags are read once and kept for the review")
	assert.Empty(t, opts.NewTargetTagsForTest())
}

// A worker has no target tags at all.
func TestPromptMissing_WorkerIsNotAskedForTargetTags(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedWorkerFlags()
	flags.StorageClass.Value = "standard"
	flags.ReadWriteMany.Value = true

	opts := newOptions(t, flags, agentK8s.ModeWorker, asker)
	opts.TargetTagsCallback = func() ([]string, error) {
		t.Fatal("a worker has no target tags")
		return nil, nil
	}

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
}

func TestBuildValues_ScriptPodPermissions(t *testing.T) {
	opts := completedTargetOptions(t)
	assert.NotContains(t, mustBuild(t, opts), "scriptPods", "the chart's own default is left alone")

	opts.RestrictScriptPods.Value = true
	clusterRole := mustBuild(t, opts)["scriptPods"].(map[string]any)["serviceAccount"].(map[string]any)["clusterRole"].(map[string]any)
	assert.Equal(t, false, clusterRole["enabled"])
	assert.NotContains(t, clusterRole, "rules")

	opts.RestrictScriptPods.Value = false
	opts.ScriptPodRules = []any{map[string]any{"apiGroups": []string{"apps"}, "resources": []string{"deployments"}, "verbs": []string{"get"}}}
	clusterRole = mustBuild(t, opts)["scriptPods"].(map[string]any)["serviceAccount"].(map[string]any)["clusterRole"].(map[string]any)
	assert.Equal(t, opts.ScriptPodRules, clusterRole["rules"])
	assert.NotContains(t, clusterRole, "enabled", "the role still has to exist for the rules to go in")
}

// A role named by flag has to be read whichever way the install was started,
// or the review would promise permissions the chart never receives.
func TestPromptMissing_ResolvesAScriptPodRoleGivenByFlag(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ScriptPodRoles.Value = []string{"deployer"}
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.PermissionsController = true

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Len(t, opts.ScriptPodRules, 1, "the rules are read even though nothing was asked")
}
