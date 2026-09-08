package apiclient_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableTLSVerification_WithNoTransport(t *testing.T) {
	client := &http.Client{}

	require.NoError(t, apiclient.DisableTLSVerification(client))

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "expected a plain transport, got %T", client.Transport)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestDisableTLSVerification_WithAPlainTransport(t *testing.T) {
	original := &http.Transport{TLSClientConfig: &tls.Config{ServerName: "example.invalid"}}
	client := &http.Client{Transport: original}

	require.NoError(t, apiclient.DisableTLSVerification(client))

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "expected a plain transport, got %T", client.Transport)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.Equal(t, "example.invalid", transport.TLSClientConfig.ServerName, "the rest of the TLS config should be carried over")
}

// The transport being replaced is usually http.DefaultTransport, which the rest
// of the process shares, so it must not be turned insecure in place.
func TestDisableTLSVerification_DoesNotMutateTheTransportItReplaces(t *testing.T) {
	original := &http.Transport{}
	client := &http.Client{Transport: original}

	require.NoError(t, apiclient.DisableTLSVerification(client))

	assert.NotSame(t, original, client.Transport)
	if original.TLSClientConfig != nil { // Clone fills this in as a side effect of setting up HTTP/2
		assert.False(t, original.TLSClientConfig.InsecureSkipVerify, "the shared transport must still verify certificates")
	}
}

// The client built by NewClientFactoryFromConfig wraps its transport in a
// SpinnerRoundTripper. Asserting the transport type instead of unwrapping it
// panicked `login --ignore-ssl-errors` for anyone with a server configured.
func TestDisableTLSVerification_WithASpinnerRoundTripper(t *testing.T) {
	client := &http.Client{Transport: apiclient.NewSpinnerRoundTripper(&fakeAskProvider{interactive: false})}

	require.NoError(t, apiclient.DisableTLSVerification(client))

	roundTripper, ok := client.Transport.(*apiclient.SpinnerRoundTripper)
	require.True(t, ok, "the spinner should be kept, got %T", client.Transport)
	transport, ok := roundTripper.Next.(*http.Transport)
	require.True(t, ok, "expected a plain transport underneath, got %T", roundTripper.Next)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestDisableTLSVerification_WithAnUnsupportedTransport(t *testing.T) {
	original := &recordingTransport{}
	client := &http.Client{Transport: original}

	err := apiclient.DisableTLSVerification(client)

	assert.ErrorContains(t, err, "unsupported HTTP transport")
	assert.Same(t, original, client.Transport, "the client should be left alone")
}

// End to end over TLS: a self-signed certificate is exactly what
// --ignore-ssl-errors exists for.
func TestDisableTLSVerification_LetsASelfSignedServerThrough(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	roundTripper := apiclient.NewSpinnerRoundTripper(&fakeAskProvider{interactive: false})
	roundTripper.Next = &http.Transport{} // rather than http.DefaultTransport, which other code in this package turns insecure
	client := &http.Client{Transport: roundTripper}

	_, err := client.Get(server.URL)
	require.Error(t, err, "the self-signed certificate should be rejected to start with")

	require.NoError(t, apiclient.DisableTLSVerification(client))

	response, err := client.Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
}
