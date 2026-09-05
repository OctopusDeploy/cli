package argocd_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// argoSecret stands in for Argo CD's own Secret, which holds its TLS and
// session signing keys alongside anything Octopus writes there.
func argoSecret(data map[string][]byte) *corev1.Secret {
	base := map[string][]byte{
		"server.secretkey": []byte("signing-key"),
		"tls.crt":          []byte("cert"),
		"tls.key":          []byte("key"),
		"admin.password":   []byte("$2a$10$adminhash"),
	}
	for k, v := range data {
		base[k] = v
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: argocd.SecretName, Namespace: "argocd"},
		Data:       base,
	}
}

func bootstrapSpec() argocd.AccountSpec {
	return argocd.AccountSpec{Name: "octopus", AllowSync: true}
}

func TestDiagnoseAuth_InitialSecretPresent(t *testing.T) {
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: argocd.InitialAdminSecretName, Namespace: "argocd"},
			Data:       map[string][]byte{"password": []byte("hunter2")},
		},
	)

	diagnosis, err := argocd.DiagnoseAuth(context.Background(), c, inClusterInstance())
	require.NoError(t, err)

	assert.True(t, diagnosis.HasInitialAdminSecret)
	assert.True(t, diagnosis.AdminEnabled)
	assert.Contains(t, diagnosis.Explain(), "available")
}

// Argo CD tells administrators to delete the initial secret once they have
// logged in, so this is the ordinary state of an established installation.
func TestDiagnoseAuth_InitialSecretDeleted(t *testing.T) {
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}))

	diagnosis, err := argocd.DiagnoseAuth(context.Background(), c, inClusterInstance())
	require.NoError(t, err)

	assert.False(t, diagnosis.HasInitialAdminSecret)
	assert.Contains(t, diagnosis.Explain(), argocd.InitialAdminSecretName)
}

func TestDiagnoseAuth_AdminLoginDisabled(t *testing.T) {
	c := clusterWith(cm(argocd.ConfigMapName, map[string]string{
		"accounts.octopus": "apiKey",
		"admin.enabled":    "false",
	}))

	diagnosis, err := argocd.DiagnoseAuth(context.Background(), c, inClusterInstance())
	require.NoError(t, err)

	assert.False(t, diagnosis.AdminEnabled)
	assert.Contains(t, diagnosis.Explain(), "admin login is disabled")
}

func TestBootstrapLogin_SetsAUsablePasswordThenRemovesIt(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		argoSecret(nil),
	)

	bootstrap, err := argocd.BeginBootstrapLogin(ctx, c, inClusterInstance(), bootstrapSpec())
	require.NoError(t, err)

	credentials := bootstrap.Credentials()
	assert.Equal(t, "octopus", credentials.Username)
	assert.NotEmpty(t, credentials.Password)

	// The stored hash has to match the password handed back, or the sign-in
	// that follows cannot work.
	hash, found, err := c.SecretKey(ctx, "argocd", argocd.SecretName, "accounts.octopus.password")
	require.NoError(t, err)
	require.True(t, found)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(credentials.Password)))

	// Signing in needs the login capability, which the account did not have.
	config, _, err := c.GetConfigMap(ctx, "argocd", argocd.ConfigMapName)
	require.NoError(t, err)
	assert.Contains(t, config.Data["accounts.octopus"], "login")

	require.NoError(t, bootstrap.Revert(ctx))

	_, found, err = c.SecretKey(ctx, "argocd", argocd.SecretName, "accounts.octopus.password")
	require.NoError(t, err)
	assert.False(t, found, "the temporary password must not be left behind")

	config, _, err = c.GetConfigMap(ctx, "argocd", argocd.ConfigMapName)
	require.NoError(t, err)
	assert.Equal(t, "apiKey", config.Data["accounts.octopus"], "the login capability must be handed back")
}

// Argo CD's own keys live in the same Secret. Losing them would break the
// installation, so the write has to be key by key.
func TestBootstrapLogin_LeavesArgoCDsOwnKeysAlone(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
		argoSecret(nil),
	)

	bootstrap, err := argocd.BeginBootstrapLogin(ctx, c, inClusterInstance(), bootstrapSpec())
	require.NoError(t, err)
	require.NoError(t, bootstrap.Revert(ctx))

	secret, found, err := c.GetSecret(ctx, "argocd", argocd.SecretName)
	require.NoError(t, err)
	require.True(t, found)

	for key, want := range map[string]string{
		"server.secretkey": "signing-key",
		"tls.crt":          "cert",
		"tls.key":          "key",
		"admin.password":   "$2a$10$adminhash",
	} {
		assert.Equal(t, want, string(secret.Data[key]), "%s must survive untouched", key)
	}
}

func TestBootstrapLogin_RestoresAPasswordThatWasAlreadyThere(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey, login"}),
		argoSecret(map[string][]byte{
			"accounts.octopus.password":      []byte("$2a$10$originalhash"),
			"accounts.octopus.passwordMtime": []byte("2026-01-01T00:00:00Z"),
		}),
	)

	bootstrap, err := argocd.BeginBootstrapLogin(ctx, c, inClusterInstance(), bootstrapSpec())
	require.NoError(t, err)

	hash, _, err := c.SecretKey(ctx, "argocd", argocd.SecretName, "accounts.octopus.password")
	require.NoError(t, err)
	assert.NotEqual(t, "$2a$10$originalhash", hash, "a temporary password is in force during the bootstrap")

	require.NoError(t, bootstrap.Revert(ctx))

	hash, found, err := c.SecretKey(ctx, "argocd", argocd.SecretName, "accounts.octopus.password")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "$2a$10$originalhash", hash, "the original password must be put back exactly")

	mtime, _, err := c.SecretKey(ctx, "argocd", argocd.SecretName, "accounts.octopus.passwordMtime")
	require.NoError(t, err)
	assert.Equal(t, "2026-01-01T00:00:00Z", mtime)
}

func TestBootstrapLogin_DoesNotRevokeACapabilityItDidNotGrant(t *testing.T) {
	ctx := context.Background()
	c := clusterWith(
		cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey, login"}),
		argoSecret(nil),
	)

	bootstrap, err := argocd.BeginBootstrapLogin(ctx, c, inClusterInstance(), bootstrapSpec())
	require.NoError(t, err)
	require.NoError(t, bootstrap.Revert(ctx))

	config, _, err := c.GetConfigMap(ctx, "argocd", argocd.ConfigMapName)
	require.NoError(t, err)
	assert.Contains(t, config.Data["accounts.octopus"], "login")
	assert.Contains(t, config.Data["accounts.octopus"], "apiKey")
}

func TestBootstrapLogin_PasswordIsNotReused(t *testing.T) {
	ctx := context.Background()

	passwords := map[string]bool{}
	for range 5 {
		c := clusterWith(
			cm(argocd.ConfigMapName, map[string]string{"accounts.octopus": "apiKey"}),
			argoSecret(nil),
		)
		bootstrap, err := argocd.BeginBootstrapLogin(ctx, c, inClusterInstance(), bootstrapSpec())
		require.NoError(t, err)
		passwords[bootstrap.Credentials().Password] = true
	}

	assert.Len(t, passwords, 5)
}
