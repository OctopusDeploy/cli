package login

import (
	"net/http"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/stretchr/testify/assert"
)

func TestSkipTlsVerification_OnATransport(t *testing.T) {
	transport := &http.Transport{}

	assert.NoError(t, skipTlsVerification(transport))
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

// The transport the client factory hands out is always spinner-wrapped, which is
// what the old type assertion tripped over.
func TestSkipTlsVerification_ThroughTheSpinner(t *testing.T) {
	spinner := apiclient.NewSpinnerRoundTripper(nil)
	spinner.Next = &http.Transport{}

	assert.NoError(t, skipTlsVerification(spinner))
	assert.True(t, spinner.Next.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
}

func TestSkipTlsVerification_ReportsAnUnknownTransport(t *testing.T) {
	err := skipTlsVerification(unknownTransport{})

	assert.ErrorContains(t, err, "unsupported HTTP transport login.unknownTransport")
}

type unknownTransport struct{}

func (unknownTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}
