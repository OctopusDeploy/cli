package install_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acceptOnly refuses every credential except the one it is given, standing in
// for an Argo CD that no longer accepts the initial admin password.
type acceptOnly struct {
	password string
	tried    []string
}

func (a *acceptOnly) Login(_ context.Context, credentials argocd.Credentials) error {
	a.tried = append(a.tried, credentials.Username)
	if credentials.Password != a.password {
		return errors.New("Argo CD returned 401 Unauthorized: Invalid username or password")
	}
	return nil
}

func strategy(name, username, password string, reverted *bool) install.LoginStrategy {
	return install.LoginStrategy{
		Describe: name,
		Begin: func(context.Context) (argocd.Credentials, func(), error) {
			return argocd.Credentials{Username: username, Password: password},
				func() { *reverted = true }, nil
		},
	}
}

// Argo CD leaves argocd-initial-admin-secret in place when the admin password
// is changed, so a rejection there must not end the whole install.
func TestSignIn_MovesOnWhenACredentialIsRejected(t *testing.T) {
	client := &acceptOnly{password: "the-real-one"}
	var firstReverted, secondReverted bool

	revert, err := install.SignIn(context.Background(), io.Discard, client, []install.LoginStrategy{
		strategy("the initial admin password", "admin", "stale", &firstReverted),
		strategy("a temporary password on the octopus account", "octopus", "the-real-one", &secondReverted),
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "octopus"}, client.tried)

	// A strategy that was tried and turned away has to undo itself; the one
	// that worked is undone by the caller once it is finished with.
	assert.True(t, firstReverted, "a rejected strategy must clean up after itself")
	assert.False(t, secondReverted)

	require.NotNil(t, revert)
	revert()
	assert.True(t, secondReverted)
}

// A strategy that cannot even produce a credential is skipped just the same.
func TestSignIn_MovesOnWhenAStrategyCannotStart(t *testing.T) {
	client := &acceptOnly{password: "the-real-one"}
	var reverted bool

	_, err := install.SignIn(context.Background(), io.Discard, client, []install.LoginStrategy{
		{
			Describe: "the initial admin password",
			Begin: func(context.Context) (argocd.Credentials, func(), error) {
				return argocd.Credentials{}, nil, errors.New("the secret is not present")
			},
		},
		strategy("a temporary password", "octopus", "the-real-one", &reverted),
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"octopus"}, client.tried)
}

// When nothing is accepted, every attempt is reported rather than just the last.
func TestSignIn_ReportsEveryAttempt(t *testing.T) {
	client := &acceptOnly{password: "nothing matches this"}
	var reverted bool

	_, err := install.SignIn(context.Background(), io.Discard, client, []install.LoginStrategy{
		strategy("the initial admin password", "admin", "stale", &reverted),
		{
			Describe: "a temporary password on the octopus account",
			Begin: func(context.Context) (argocd.Credentials, func(), error) {
				return argocd.Credentials{}, nil, errors.New("this Argo CD is managed by an operator")
			},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the initial admin password")
	assert.Contains(t, err.Error(), "Invalid username or password")
	assert.Contains(t, err.Error(), "managed by an operator")
	assert.True(t, reverted)
}

func TestSignIn_StopsAtTheFirstThatWorks(t *testing.T) {
	client := &acceptOnly{password: "the-real-one"}
	var firstReverted, secondReverted bool

	_, err := install.SignIn(context.Background(), io.Discard, client, []install.LoginStrategy{
		strategy("the initial admin password", "admin", "the-real-one", &firstReverted),
		strategy("a temporary password", "octopus", "the-real-one", &secondReverted),
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"admin"}, client.tried, "no further credentials should be tried")
	assert.False(t, firstReverted)
}
