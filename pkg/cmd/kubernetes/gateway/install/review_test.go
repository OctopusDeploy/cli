package install_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewOf(t *testing.T, opts *install.InstallOptions) string {
	t.Helper()

	out := &bytes.Buffer{}
	opts.Out = out
	install.RenderReviewForDemo(opts)
	return out.String()
}

func reviewOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := install.NewInstallFlags()
	flags.Name.Value = "Production"
	flags.Environments.Value = []string{"production", "staging"}
	flags.KubeContext.Value = "colima-k8s"
	flags.OctopusGRPCURL.Value = "grpc://my.octopus.app:8443"
	flags.ArgoCDServerGRPCURL.Value = "grpc://argocd-server.argocd.svc.cluster.local"
	flags.ArgoCDToken.Value = "eyJhbGciOiJIUzI1NiJ9.super-secret-token"
	flags.ArgoCDAccountName.Value = argocd.DefaultAccountName

	space := &spaces.Space{Name: "Default"}
	space.ID = "Spaces-1"

	opts := &install.InstallOptions{
		InstallFlags: flags,
		Dependencies: &cmd.Dependencies{Ask: asker, Out: &bytes.Buffer{}, Host: "https://my.octopus.app", Space: space},
		Instance:     stockInstance(),
		KubeContextInfo: octoK8s.Context{
			Name: "colima-k8s", Server: "https://192.168.64.4:52409", IsCurrent: true,
		},
	}
	return opts
}

// Almost everything here is worked out rather than asked for, so the review is
// the only place a person sees it before anything is created.
func TestReview_ShowsEveryDetectedSetting(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	for _, expected := range []string{
		"colima-k8s",                   // chosen cluster
		"https://192.168.64.4:52409",   // detected cluster address
		"octo-argo-gateway-production", // derived namespace
		"production",                   // derived release name
		"https://my.octopus.app",       // from the login
		"Default",                      // space
		"grpc://my.octopus.app:8443",   // derived
		"grpc://argocd-server.argocd.svc.cluster.local", // detected
		"v3.4.2",                               // detected Argo CD version
		"TLS, certificate not verified",        // detected TLS posture
		"Argo CD token in a Kubernetes Secret", // where the credential goes
	} {
		assert.Contains(t, review, expected)
	}
}

// Registering from Octopus rather than from the cluster is a security property
// worth stating on the screen someone approves.
func TestReview_SaysNoOctopusCredentialIsStoredInTheCluster(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	assert.Contains(t, review, "Registration")
	assert.Contains(t, review, "no Octopus credential is stored in the cluster")
}

// The review is printed to the terminal, so a token must not appear in it.
func TestReview_MasksTheToken(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	assert.NotContains(t, review, "super-secret-token")
	assert.Contains(t, review, "***")
}

// Saying where a value came from is what lets someone spot a wrong guess.
func TestReview_SaysWhereValuesCameFrom(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	assert.Contains(t, review, "(derived from the name)")
	assert.Contains(t, review, "(found in the cluster)")
	assert.Contains(t, review, "(from your login)")
}

// Claiming a source for something that was never found reads as a detection
// that succeeded.
func TestReview_ClaimsNoSourceForAnUnsetValue(t *testing.T) {
	review := reviewOf(t, reviewOptions(t))

	for _, line := range strings.Split(review, "\n") {
		if strings.Contains(line, "Web UI") {
			assert.Contains(t, line, "(not set)")
			assert.NotContains(t, line, "found in the cluster")
		}
	}
}

func TestReview_ManagedArgoCDShowsProjectTokensNotAnAccount(t *testing.T) {
	opts := reviewOptions(t)
	opts.Instance = argocd.NewManagedInstance("abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com")
	opts.ArgoCDServerGRPCURL.Value = opts.Instance.ServerGRPCURL
	opts.ArgoCDProjectTokens.Value = []string{"default=token-a", "team-a=token-b"}

	review := reviewOf(t, opts)

	assert.Contains(t, review, "AWS managed")
	assert.Contains(t, review, "gRPC-Web")
	assert.Contains(t, review, "default, team-a", "the projects are listed")
	assert.NotContains(t, review, "token-a", "the tokens themselves are not")
	assert.NotContains(t, review, "Account", "a managed instance has no Octopus-managed account")
}

func TestReview_DistinguishesAnExplicitNamespace(t *testing.T) {
	opts := reviewOptions(t)
	opts.Namespace.Value = "my-own-namespace"

	review := reviewOf(t, opts)

	assert.Contains(t, review, "my-own-namespace")
	for _, line := range strings.Split(review, "\n") {
		if strings.Contains(line, "Namespace") {
			assert.NotContains(t, line, "derived")
		}
	}
}

// Linking at the instance leaves someone to find the project and role
// themselves, which is the whole point of the link.
func TestPromptForProjectToken_LinksAtTheRoleNotTheInstance(t *testing.T) {
	const endpoint = "abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com"

	out := &bytes.Buffer{}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewPasswordPrompt("Token for project team-a", "", projectJWT(t, "proj:team-a:octopus")),
	})

	opts := reviewOptions(t)
	opts.Ask = asker
	opts.Out = out
	opts.Instance = argocd.NewManagedInstance(endpoint)
	opts.Instance.WebUIURL = "https://" + endpoint

	require.NoError(t, install.PromptForProjectTokenForTest(opts, "team-a"))
	checkRemainingPrompts()

	assert.Contains(t, out.String(),
		"https://"+endpoint+"/settings/projects/team-a?editRole=octopus&tab=roles")
	assert.Contains(t, out.String(), "argocd proj role create-token team-a octopus")
}

// The account name is defaulted during discovery, but a blank one must not
// silently degrade the link back to the instance.
func TestPromptForProjectToken_LinksCorrectlyWithoutAnExplicitAccountName(t *testing.T) {
	out := &bytes.Buffer{}
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewPasswordPrompt("Token for project default", "", projectJWT(t, "proj:default:octopus")),
	})

	opts := reviewOptions(t)
	opts.Ask = asker
	opts.Out = out
	opts.ArgoCDAccountName.Value = ""
	opts.Instance = argocd.NewManagedInstance("x.example.com")
	opts.Instance.WebUIURL = "https://x.example.com"

	require.NoError(t, install.PromptForProjectTokenForTest(opts, "default"))

	assert.Contains(t, out.String(), "?editRole=octopus&tab=roles")
}

// A token pasted for the wrong project is caught rather than stored.
func TestPromptForProjectToken_RejectsATokenForAnotherProject(t *testing.T) {
	out := &bytes.Buffer{}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, []*testutil.PA{
		testutil.NewPasswordPrompt("Token for project team-a", "", projectJWT(t, "proj:team-b:octopus")),
		testutil.NewPasswordPrompt("Token for project team-a", "", projectJWT(t, "proj:team-a:octopus")),
	})

	opts := reviewOptions(t)
	opts.Ask = asker
	opts.Out = out
	opts.Instance = argocd.NewManagedInstance("x.example.com")
	opts.Instance.WebUIURL = "https://x.example.com"

	require.NoError(t, install.PromptForProjectTokenForTest(opts, "team-a"))
	checkRemainingPrompts()

	assert.Contains(t, out.String(), "That token is for project")
	require.Len(t, opts.ArgoCDProjectTokens.Value, 1)
}

func projectJWT(t *testing.T, subject string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"iss": "argocd", "sub": subject})
	require.NoError(t, err)

	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc(payload) + ".c2ln"
}
