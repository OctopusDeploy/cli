package argocd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

const (
	accountPasswordKeySuffix      = ".password"
	accountPasswordMtimeKeySuffix = ".passwordMtime"
	accountsKeyPrefix             = "accounts."
)

type AuthDiagnosis struct {
	// Argo CD tells administrators to delete the initial admin Secret once they
	// have logged in, so an established installation usually has none.
	HasInitialAdminSecret bool
	// Commonly false once an installation is wired up to an identity provider.
	AdminEnabled bool
}

func (d AuthDiagnosis) Explain() string {
	switch {
	case !d.AdminEnabled:
		return "admin login is disabled on this Argo CD (admin.enabled is false in " + ConfigMapName + ")"
	case !d.HasInitialAdminSecret:
		return "the " + InitialAdminSecretName + " Secret is no longer present, which is normal once someone has logged in to Argo CD"
	default:
		return "the initial admin password is available, though Argo CD leaves that Secret in place when the admin " +
			"password is changed, so it may no longer be the right one"
	}
}

// DiagnoseAuth lets a failure say what is actually wrong rather than just that
// login failed.
func DiagnoseAuth(ctx context.Context, c *octoK8s.Cluster, instance Instance) (AuthDiagnosis, error) {
	namespace := instance.Namespace
	diagnosis := AuthDiagnosis{AdminEnabled: true}

	_, found, err := c.GetSecret(ctx, namespace, InitialAdminSecretName)
	if err != nil {
		return AuthDiagnosis{}, err
	}
	diagnosis.HasInitialAdminSecret = found

	cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName)
	if err != nil {
		return AuthDiagnosis{}, err
	}
	if found && strings.EqualFold(strings.TrimSpace(cm.Data["admin.enabled"]), "false") {
		diagnosis.AdminEnabled = false
	}

	return diagnosis, nil
}

// BootstrapLogin obtains a token when no administrator password is available.
// Argo CD keeps local account passwords in argocd-secret, so cluster access is
// enough to set one; signing in as the Octopus account means Argo CD mints the
// token itself, so nothing is forged and the administrator's password is never
// read or changed.
//
// Revert must be called to undo it.
type BootstrapLogin struct {
	cluster     *octoK8s.Cluster
	namespace   string
	accountName string
	password    string

	// Captured before anything changed, to put back afterwards.
	grantedLogin     bool
	previousPassword *string
	previousMtime    *string
}

func (b *BootstrapLogin) Credentials() Credentials {
	return Credentials{Username: b.accountName, Password: b.password}
}

// BeginBootstrapLogin grants a temporary password and, if the account lacks it,
// the ability to log in.
func BeginBootstrapLogin(ctx context.Context, c *octoK8s.Cluster, instance Instance, spec AccountSpec) (*BootstrapLogin, error) {
	namespace := instance.Namespace

	// An operator reconciles argocd-secret from its own resource, so a password
	// written here would be taken away again, possibly mid-sign-in.
	if instance.Operator != nil {
		return nil, fmt.Errorf(
			"this Argo CD is managed by an operator (the %s ArgoCD resource), which regenerates its Secret, "+
				"so Octopus cannot give an account a temporary password here", instance.Operator.Name)
	}

	password, err := randomPassword()
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("could not prepare a temporary Argo CD password: %w", err)
	}

	bootstrap := &BootstrapLogin{
		cluster:     c,
		namespace:   namespace,
		accountName: spec.Name,
		password:    password,
	}

	granted, err := grantLoginCapability(ctx, c, namespace, spec.Name)
	if err != nil {
		return nil, err
	}
	bootstrap.grantedLogin = granted

	// Put back exactly whatever was there, rather than replacing it with nothing.
	passwordKey := accountsKeyPrefix + spec.Name + accountPasswordKeySuffix
	mtimeKey := accountsKeyPrefix + spec.Name + accountPasswordMtimeKeySuffix

	if secret, found, err := c.GetSecret(ctx, namespace, SecretName); err != nil {
		return nil, err
	} else if found {
		if value, ok := secret.Data[passwordKey]; ok {
			previous := string(value)
			bootstrap.previousPassword = &previous
		}
		if value, ok := secret.Data[mtimeKey]; ok {
			previous := string(value)
			bootstrap.previousMtime = &previous
		}
	}

	err = c.MergeSecretKeys(ctx, namespace, SecretName, map[string]string{
		passwordKey: string(hash),
		mtimeKey:    time.Now().UTC().Format(time.RFC3339),
	}, nil)
	if err != nil {
		_ = bootstrap.Revert(ctx)
		return nil, err
	}

	return bootstrap, nil
}

// Revert leaves any token minted in between working: an API token is validated
// against the account's token list, not its password.
func (b *BootstrapLogin) Revert(ctx context.Context) error {
	passwordKey := accountsKeyPrefix + b.accountName + accountPasswordKeySuffix
	mtimeKey := accountsKeyPrefix + b.accountName + accountPasswordMtimeKeySuffix

	set := map[string]string{}
	var remove []string

	if b.previousPassword != nil {
		set[passwordKey] = *b.previousPassword
	} else {
		remove = append(remove, passwordKey)
	}
	if b.previousMtime != nil {
		set[mtimeKey] = *b.previousMtime
	} else {
		remove = append(remove, mtimeKey)
	}

	if err := b.cluster.MergeSecretKeys(ctx, b.namespace, SecretName, set, remove); err != nil {
		return err
	}

	if b.grantedLogin {
		return revokeLoginCapability(ctx, b.cluster, b.namespace, b.accountName)
	}
	return nil
}

// grantLoginCapability reports whether it had to, so reverting does not take
// away a capability the user configured themselves.
func grantLoginCapability(ctx context.Context, c *octoK8s.Cluster, namespace, accountName string) (bool, error) {
	cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("ConfigMap %s/%s does not exist", namespace, ConfigMapName)
	}

	key := accountsKeyPrefix + accountName
	if containsCapability(cm.Data[key], capabilityLogin) {
		return false, nil
	}

	updated := cm.DeepCopy()
	updated.Data[key] = addCapability(updated.Data[key], capabilityLogin)
	if err := updateConfigMap(ctx, c, updated); err != nil {
		return false, err
	}
	return true, nil
}

func revokeLoginCapability(ctx context.Context, c *octoK8s.Cluster, namespace, accountName string) error {
	cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName)
	if err != nil || !found {
		return err
	}

	key := accountsKeyPrefix + accountName
	updated := cm.DeepCopy()
	updated.Data[key] = removeCapability(updated.Data[key], capabilityLogin)
	return updateConfigMap(ctx, c, updated)
}

func randomPassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate a temporary Argo CD password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
