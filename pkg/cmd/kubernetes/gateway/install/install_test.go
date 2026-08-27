package install_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const octopusHost = "https://my.octopus.app"

func devEnvironment() *environments.Environment {
	e := environments.NewEnvironment("Development")
	e.ID = "Environments-1"
	e.Slug = "development"
	return e
}

func prodEnvironment() *environments.Environment {
	e := environments.NewEnvironment("Production")
	e.ID = "Environments-2"
	e.Slug = "production"
	return e
}

// stockInstance is what discovery reports for a default Argo CD install: TLS
// on, with Argo CD's own self-signed certificate.
func stockInstance() argocd.Instance {
	return argocd.Instance{
		Namespace:     "argocd",
		ServiceName:   "argocd-server",
		Version:       "v3.4.2",
		ServerGRPCURL: "grpc://argocd-server.argocd.svc.cluster.local",
		Plaintext:     false,
		SelfSignedTLS: true,
	}
}

// configuredArgoCD is a cluster whose Argo CD already has the octopus account
// and RBAC in place, so the installer only has to ask for a token.
func configuredArgoCD(objects ...runtime.Object) *octoK8s.Cluster {
	base := []runtime.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: argocd.ConfigMapName, Namespace: "argocd"},
			Data:       map[string]string{"accounts.octopus": "apiKey"},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: argocd.RBACConfigMapName, Namespace: "argocd"},
			Data: map[string]string{"policy.csv": `
p, octopus, applications, get, *, allow
p, octopus, applications, sync, *, allow
p, octopus, clusters, get, *, allow
p, octopus, logs, get, */*, allow
`},
		},
	}
	return octoK8s.NewClusterForTesting(fake.NewSimpleClientset(append(base, objects...)...), "test", "https://cluster")
}

func newOptions(t *testing.T, flags *install.InstallFlags, asker func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error) *install.InstallOptions {
	t.Helper()

	opts := &install.InstallOptions{
		InstallFlags: flags,
		Dependencies: &cmd.Dependencies{
			Ask:   asker,
			Out:   &bytes.Buffer{},
			Host:  octopusHost,
			Space: &spaces.Space{Name: "Default"},
		},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{devEnvironment(), prodEnvironment()}, nil
		},
		Cluster:   configuredArgoCD(),
		Instances: []argocd.Instance{stockInstance()},
	}
	opts.Space.ID = "Spaces-1"
	return opts
}

func TestPromptMissing_NoOptionsSupplied(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this Argo CD instance.", "Production"),
		testutil.NewMultiSelectPrompt("Which environments does this Argo CD instance serve?", "",
			[]string{"Development", "Production"}, []string{"Production"}),
		testutil.NewInputPromptWithDefault("Octopus Server gRPC address",
			"The gateway holds a gRPC connection to Octopus on port 8443, separate from the REST API on 443. "+
				"If Octopus sits behind a load balancer or proxy, port 8443 must be forwarded to it.",
			"grpc://my.octopus.app:8443", "grpc://my.octopus.app:8443"),
		testutil.NewConfirmPromptWithDefault(`Generate an Argo CD token for the "octopus" account?`,
			"Octopus needs an Argo CD token to read applications and clusters.", false, true),
		testutil.NewPasswordPrompt("Argo CD authentication token",
			"A JWT for an Argo CD account that can read applications, clusters and logs.", "eyJhbGciOiJIUzI1NiJ9.token"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := install.NewInstallFlags()
	flags.ArgoCDAccountName.Value = argocd.DefaultAccountName
	flags.AllowSync.Value = true
	opts := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, "Production", flags.Name.Value)
	assert.Equal(t, []string{"production"}, flags.Environments.Value, "the environment slug is used, since it survives a rename")
	assert.Equal(t, "grpc://my.octopus.app:8443", flags.OctopusGRPCURL.Value)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.token", flags.ArgoCDToken.Value)

	// Everything below was discovered rather than asked for.
	assert.Equal(t, "argocd", flags.ArgoCDNamespace.Value)
	assert.Equal(t, "grpc://argocd-server.argocd.svc.cluster.local", flags.ArgoCDServerGRPCURL.Value)
	assert.Equal(t, "octo-argo-gateway-production", opts.TargetNamespace)
	assert.Equal(t, "production", opts.TargetRelease)
}

func TestPromptMissing_AllOptionsSupplied(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.Name.Value = "Production"
	flags.Environments.Value = []string{"production"}
	flags.ArgoCDNamespace.Value = "argocd"
	flags.ArgoCDServerGRPCURL.Value = "grpc://argocd-server.argocd.svc.cluster.local"
	flags.ArgoCDToken.Value = "eyJhbGciOiJIUzI1NiJ9.token"
	flags.OctopusGRPCURL.Value = "grpc://my.octopus.app:8443"
	flags.ArgoCDAccountName.Value = argocd.DefaultAccountName

	opts := newOptions(t, flags, asker)

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()
}

func TestPromptMissing_DerivesNamespaceFromName(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.Name.Value = "EU West Production"
	flags.Environments.Value = []string{"production"}
	flags.ArgoCDNamespace.Value = "argocd"
	flags.ArgoCDToken.Value = "token"
	flags.OctopusGRPCURL.Value = "grpc://my.octopus.app:8443"
	flags.ArgoCDAccountName.Value = argocd.DefaultAccountName

	opts := newOptions(t, flags, asker)
	require.NoError(t, install.PromptMissing(context.Background(), opts))

	assert.Equal(t, "octo-argo-gateway-eu-west-production", opts.TargetNamespace)
	assert.Equal(t, "eu-west-production", opts.TargetRelease)
}

// A cluster running Argo CD in insecure mode needs the opposite TLS settings to
// a stock install, and getting them wrong is a documented cause of a gateway
// that installs and then never connects.
func TestBuildValues_TLSSettingsFollowTheCluster(t *testing.T) {
	tests := []struct {
		name          string
		instance      argocd.Instance
		wantPlaintext bool
		wantInsecure  bool
	}{
		{"stock install serves self-signed TLS", stockInstance(), false, true},
		{"insecure mode serves no TLS", argocd.Instance{
			Namespace:     "argocd",
			ServerGRPCURL: "grpc://argocd-server.argocd.svc.cluster.local",
			Plaintext:     true,
			SelfSignedTLS: false,
		}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := completedOptions(t)
			opts.Instance = tt.instance

			values, err := opts.BuildValues()
			require.NoError(t, err)

			argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)
			assert.Equal(t, tt.wantPlaintext, argo["plaintext"])
			assert.Equal(t, tt.wantInsecure, argo["insecure"])
		})
	}
}

func TestBuildValues_ArgoCDTokenGoesIntoASecretByDefault(t *testing.T) {
	opts := completedOptions(t)

	values, err := opts.BuildValues()
	require.NoError(t, err)

	argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)
	assert.Equal(t, "octopus-argocd-gateway-argocd-token", argo["authenticationTokenSecretName"])
	assert.Equal(t, "ARGOCD_AUTH_TOKEN", argo["authenticationTokenSecretKey"])
	assert.NotContains(t, argo, "authenticationToken", "the Argo CD JWT must not reach the Helm values")
}

// Octopus registers the gateway before the chart is installed, so the chart has
// no reason to hold an Octopus credential and no reason to register itself.
func TestBuildValues_NoOctopusCredentialReachesTheCluster(t *testing.T) {
	opts := completedOptions(t)
	opts.InlineSecrets.Value = true // even here, there is nothing to inline

	values, err := opts.BuildValues()
	require.NoError(t, err)

	registration := values["registration"].(map[string]any)
	assert.Equal(t, false, registration["register"], "the chart must not register itself")

	octopus := registration["octopus"].(map[string]any)
	for _, key := range []string{"serverAccessToken", "serverAccessTokenSecretName", "serverAccessTokenSecretKey"} {
		assert.NotContains(t, octopus, key)
	}
}

func TestBuildValues_InlineSecretsOptsIn(t *testing.T) {
	opts := completedOptions(t)
	opts.InlineSecrets.Value = true

	values, err := opts.BuildValues()
	require.NoError(t, err)

	argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)

	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.token", argo["authenticationToken"])
	assert.NotContains(t, argo, "authenticationTokenSecretName")
}

func TestBuildValues_RegistrationDetailsComeFromOctopus(t *testing.T) {
	opts := completedOptions(t)

	values, err := opts.BuildValues()
	require.NoError(t, err)

	octopus := values["registration"].(map[string]any)["octopus"].(map[string]any)
	assert.Equal(t, "Production", octopus["name"])
	assert.Equal(t, octopusHost, octopus["serverApiUrl"])
	assert.Equal(t, "Spaces-1", octopus["spaceId"])
	assert.Equal(t, []string{"production"}, octopus["environments"])
}

// The web UI URL is optional, so an Argo CD that does not advertise one should
// not produce an empty registration.argocd block.
func TestBuildValues_OmitsWebUIURLWhenUnknown(t *testing.T) {
	opts := completedOptions(t)

	values, err := opts.BuildValues()
	require.NoError(t, err)
	assert.NotContains(t, values["registration"].(map[string]any), "argocd")

	opts.ArgoCDWebUIURL.Value = "https://argo.example.com"
	values, err = opts.BuildValues()
	require.NoError(t, err)
	assert.Equal(t, "https://argo.example.com",
		values["registration"].(map[string]any)["argocd"].(map[string]any)["webUiUrl"])
}

func completedOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	flags := install.NewInstallFlags()
	flags.Name.Value = "Production"
	flags.Environments.Value = []string{"production"}
	flags.ArgoCDServerGRPCURL.Value = "grpc://argocd-server.argocd.svc.cluster.local"
	flags.ArgoCDToken.Value = "eyJhbGciOiJIUzI1NiJ9.token"
	flags.OctopusGRPCURL.Value = "grpc://my.octopus.app:8443"

	opts := newOptions(t, flags, asker)
	opts.Instance = stockInstance()
	return opts
}

func managedOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})
	flags := install.NewInstallFlags()
	flags.Name.Value = "EKS Production"
	flags.Environments.Value = []string{"production"}
	flags.OctopusGRPCURL.Value = "grpc://my.octopus.app:8443"
	flags.ArgoCDServerGRPCURL.Value = "grpc://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com"

	opts := newOptions(t, flags, asker)
	opts.Instance = argocd.NewManagedInstance("abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com")
	opts.Instances = []argocd.Instance{opts.Instance}
	return opts
}

func TestBuildValues_ManagedArgoCDUsesGRPCWebAndVerifiedTLS(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)
	assert.Equal(t, true, argo["grpcWeb"], "AWS's load balancer does not support HTTP/2")
	assert.Equal(t, false, argo["insecure"], "AWS uses a publicly trusted certificate")
	assert.Equal(t, false, argo["plaintext"])
	assert.Equal(t, "grpc://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com", argo["serverGrpcUrl"])
}

func TestBuildValues_ManagedArgoCDAuthenticatesPerProject(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a", "team-a=token-b"}

	values, err := opts.BuildValues()
	require.NoError(t, err)

	argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)
	assert.Equal(t, "octopus-argocd-gateway-project-tokens", argo["projectAuthenticationSecretName"])
	assert.NotContains(t, argo, "authenticationTokenSecretName", "project tokens replace the single account token")
	assert.NotContains(t, argo, "authenticationToken")
}

func TestBuildValues_ManagedArgoCDInlineProjectTokens(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a"}
	opts.InlineSecrets.Value = true

	values, err := opts.BuildValues()
	require.NoError(t, err)

	argo := values["gateway"].(map[string]any)["argocd"].(map[string]any)
	assert.Equal(t, []argocd.ProjectToken{{Project: "default", Token: "token-a"}}, argo["projectAuthentication"])
}

func TestBuildValues_GRPCWebRootPath(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a"}
	opts.ArgoCDGRPCWebRootPath.Value = "/argo/api"

	values, err := opts.BuildValues()
	require.NoError(t, err)
	assert.Equal(t, "/argo/api", values["gateway"].(map[string]any)["argocd"].(map[string]any)["grpcWebRootPath"])
}

func TestBuildValues_InClusterOmitsGRPCWeb(t *testing.T) {
	values, err := completedOptions(t).BuildValues()
	require.NoError(t, err)
	assert.NotContains(t, values["gateway"].(map[string]any)["argocd"].(map[string]any), "grpcWeb")
}

func TestProjectTokens_Parsing(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a", " team-a = token-b "}

	tokens, err := opts.ProjectTokens()
	require.NoError(t, err)
	assert.Equal(t, []argocd.ProjectToken{
		{Project: "default", Token: "token-a"},
		{Project: "team-a", Token: "token-b"},
	}, tokens)
}

func TestProjectTokens_RejectsBadInput(t *testing.T) {
	tests := map[string][]string{
		"no separator":     {"default"},
		"empty project":    {"=token"},
		"empty token":      {"default="},
		"duplicate projet": {"default=a", "default=b"},
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			opts := managedOptions(t)
			opts.ArgoCDProjectTokens.Value = value

			_, err := opts.ProjectTokens()
			assert.Error(t, err)
		})
	}
}

// The account and RBAC automation edits argocd-cm, which managed Argo CD does
// not have.
func TestValidateForAutomation_RejectsAccountAutomationForManagedArgoCD(t *testing.T) {
	opts := managedOptions(t)
	opts.NoPrompt = true
	opts.ConfigureArgoCDAccount.Value = true

	err := opts.ResolveWithoutPrompting(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS managed Argo CD")
	assert.Contains(t, err.Error(), install.FlagArgoCDProjectToken)
}

func kubeConfigWith(t *testing.T, contexts ...string) *octoK8s.KubeConfig {
	t.Helper()

	body := "apiVersion: v1\nkind: Config\ncurrent-context: " + contexts[0] + "\ncontexts:\n"
	for _, name := range contexts {
		body += "  - name: " + name + "\n    context: {cluster: " + name + ", user: " + name + "}\n"
	}
	body += "clusters:\n"
	for _, name := range contexts {
		body += "  - name: " + name + "\n    cluster: {server: \"https://" + name + ":6443\"}\n"
	}
	body += "users:\n"
	for _, name := range contexts {
		body += "  - name: " + name + "\n    user: {token: abc}\n"
	}

	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	kubeConfig, err := octoK8s.LoadKubeConfig(path)
	require.NoError(t, err)
	return kubeConfig
}

// An expired cloud credential should not end the command: the user signs in
// again in another terminal and picks up where they were.
func TestConfirmRetry_TryAgainKeepsTheChosenCluster(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("What would you like to do?",
			"If a cloud credential helper failed, sign in again in another terminal and choose Try again.",
			[]string{"Try again", "Choose a different cluster", "Cancel"}, "Try again"),
	})

	flags := install.NewInstallFlags()
	flags.KubeContext.Value = "gke-prod"
	opts := newOptions(t, flags, asker)

	retry, err := opts.ConfirmRetry(kubeConfigWith(t, "gke-prod", "eks-prod"), errors.New("credentials expired"))
	require.NoError(t, err)

	assert.True(t, retry)
	assert.Equal(t, "gke-prod", flags.KubeContext.Value, "retrying keeps the same cluster")
	checkRemainingPrompts()
}

func TestConfirmRetry_ChoosingAnotherClusterClearsTheSelection(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("What would you like to do?",
			"If a cloud credential helper failed, sign in again in another terminal and choose Try again.",
			[]string{"Try again", "Choose a different cluster", "Cancel"}, "Choose a different cluster"),
	})

	flags := install.NewInstallFlags()
	flags.KubeContext.Value = "gke-prod"
	opts := newOptions(t, flags, asker)

	retry, err := opts.ConfirmRetry(kubeConfigWith(t, "gke-prod", "eks-prod"), errors.New("credentials expired"))
	require.NoError(t, err)

	assert.True(t, retry)
	assert.Empty(t, flags.KubeContext.Value, "clearing the selection re-opens the cluster prompt")
}

func TestConfirmRetry_Cancel(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("What would you like to do?",
			"If a cloud credential helper failed, sign in again in another terminal and choose Try again.",
			[]string{"Try again", "Choose a different cluster", "Cancel"}, "Cancel"),
	})

	opts := newOptions(t, install.NewInstallFlags(), asker)

	retry, err := opts.ConfirmRetry(kubeConfigWith(t, "gke-prod", "eks-prod"), errors.New("credentials expired"))
	require.NoError(t, err)
	assert.False(t, retry)
}

func TestConfirmRetry_SingleClusterOffersOnlyRetry(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewSelectPrompt("What would you like to do?",
			"If a cloud credential helper failed, sign in again in another terminal and choose Try again.",
			[]string{"Try again", "Cancel"}, "Cancel"),
	})

	opts := newOptions(t, install.NewInstallFlags(), asker)

	_, err := opts.ConfirmRetry(kubeConfigWith(t, "only-cluster"), errors.New("credentials expired"))
	require.NoError(t, err)
}

func TestConfirmRetry_NeverPromptsWithPromptingDisabled(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, install.NewInstallFlags(), asker)
	opts.NoPrompt = true

	retry, err := opts.ConfirmRetry(kubeConfigWith(t, "gke-prod", "eks-prod"), errors.New("credentials expired"))
	require.NoError(t, err)
	assert.False(t, retry)
	checkRemainingPrompts()
}

// A cluster with no Argo CD and nowhere else to look is a dead end, not
// something worth offering to retry.
func TestConfirmRetry_NoArgoCDOnTheOnlyCluster(t *testing.T) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{})

	opts := newOptions(t, install.NewInstallFlags(), asker)

	retry, err := opts.ConfirmRetry(kubeConfigWith(t, "only-cluster"), argocd.ErrNoInstances{})
	require.NoError(t, err)
	assert.False(t, retry)
	checkRemainingPrompts()
}

// A bare token is enough: Argo CD puts the project in the subject, so the flag
// does not need to repeat it.
func TestProjectTokens_ReadsTheProjectFromTheToken(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{
		jwtFor(t, "proj:team-a:octopus"),
		jwtFor(t, "proj:team-b:octopus"),
	}

	tokens, err := opts.ProjectTokens()
	require.NoError(t, err)
	require.Len(t, tokens, 2)
	assert.Equal(t, "team-a", tokens[0].Project)
	assert.Equal(t, "team-b", tokens[1].Project)
}

func TestProjectTokens_ExplicitProjectStillWorks(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{"team-a=opaque-token"}

	tokens, err := opts.ProjectTokens()
	require.NoError(t, err)
	assert.Equal(t, []argocd.ProjectToken{{Project: "team-a", Token: "opaque-token"}}, tokens)
}

func TestProjectTokens_RejectsTwoTokensForOneProject(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{
		jwtFor(t, "proj:team-a:octopus"),
		jwtFor(t, "proj:team-a:other"),
	}

	_, err := opts.ProjectTokens()
	assert.ErrorContains(t, err, "team-a")
}

func TestProjectTokens_RejectsAnAccountToken(t *testing.T) {
	opts := managedOptions(t)
	opts.ArgoCDProjectTokens.Value = []string{jwtFor(t, "admin:apiKey")}

	_, err := opts.ProjectTokens()
	assert.ErrorContains(t, err, "project role token")
}

func jwtFor(t *testing.T, subject string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"iss": "argocd", "sub": subject})
	require.NoError(t, err)

	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc(payload) + ".c2ln"
}
