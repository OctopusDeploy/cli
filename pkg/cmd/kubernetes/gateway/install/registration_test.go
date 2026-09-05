package install_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/argocdgateways"
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registeredOptions(t *testing.T) *install.InstallOptions {
	t.Helper()

	opts := completedOptions(t)
	opts.Registration = &argocdgateways.Registration{
		ID:                    "ArgoCDGateways-1",
		ClientID:              "3f2504e0-4f89-11d3-9a0c-0305e82c3301",
		AuthenticationToken:   "gateway-own-credential",
		CertificateThumbprint: "AABBCCDDEEFF00112233445566778899AABBCCDD",
	}
	return opts
}

// The point of registering from the CLI: the credential the chart would have
// used to register itself never has to exist in the cluster.
func TestRegistrationSecret_HoldsOnlyTheGatewaysOwnCredential(t *testing.T) {
	contents, err := registeredOptions(t).RegistrationSecretForTest()
	require.NoError(t, err)

	assert.Contains(t, contents, `octopus-grpc-authentication-token: "gateway-own-credential"`)
	assert.Contains(t, contents, `octopus-grpc-client-id: "3f2504e0-4f89-11d3-9a0c-0305e82c3301"`)
	assert.Contains(t, contents, `octopus-grpc-thumbprint: "AABBCCDDEEFF00112233445566778899AABBCCDD"`)
}

// A server that returns no thumbprint should not produce an empty setting.
func TestRegistrationSecret_OmitsAnAbsentThumbprint(t *testing.T) {
	opts := registeredOptions(t)
	opts.Registration.CertificateThumbprint = ""

	contents, err := opts.RegistrationSecretForTest()
	require.NoError(t, err)
	assert.NotContains(t, contents, "octopus-grpc-thumbprint")
	assert.Equal(t, 2, strings.Count(contents, "\n"))
}

func TestRegister_SendsTheNameSpaceAndEnvironments(t *testing.T) {
	var sent argocdgateways.RegisterCommand

	opts := completedOptions(t)
	opts.RegisterCallback = func(c argocdgateways.RegisterCommand) (*argocdgateways.Registration, error) {
		sent = c
		return &argocdgateways.Registration{ID: "ArgoCDGateways-1", AuthenticationToken: "t"}, nil
	}

	require.NoError(t, opts.RegisterForTest())

	assert.Equal(t, "Production", sent.Name)
	assert.Equal(t, "Spaces-1", sent.SpaceID)
	assert.Equal(t, []string{"production"}, sent.Environments)
}

// A registration Octopus accepted but returned no credential for would produce
// a gateway that can never connect.
func TestRegister_RejectsARegistrationWithNoCredential(t *testing.T) {
	opts := completedOptions(t)
	opts.RegisterCallback = func(argocdgateways.RegisterCommand) (*argocdgateways.Registration, error) {
		return &argocdgateways.Registration{ID: "ArgoCDGateways-1"}, nil
	}

	assert.ErrorContains(t, opts.RegisterForTest(), "no credential")
}

// Registering happens before the install, so a failed install must take the
// registration back out rather than leave a gateway that never connects.
func TestDeregister_RemovesTheRegistrationWhenTheInstallFails(t *testing.T) {
	var deleted string

	opts := registeredOptions(t)
	opts.DeregisterCallback = func(id string) error {
		deleted = id
		return nil
	}

	cause := assert.AnError
	assert.Equal(t, cause, opts.DeregisterForTest(cause), "the original failure is what the caller sees")
	assert.Equal(t, "ArgoCDGateways-1", deleted)
}

// Failing to clean up must not replace the real reason the install failed.
func TestDeregister_KeepsTheOriginalFailureWhenCleanupFails(t *testing.T) {
	opts := registeredOptions(t)
	opts.DeregisterCallback = func(string) error { return assert.AnError }

	cause := errors.New("install failed")
	assert.Equal(t, cause, opts.DeregisterForTest(cause))
	assert.Contains(t, opts.Out.(interface{ String() string }).String(), "could not be removed")
}
