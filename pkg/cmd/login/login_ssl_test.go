package login_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/stretchr/testify/assert"
)

// TestSSLIgnoreHandling tests that our SSL ignore logic works with both 
// direct http.Transport and SpinnerRoundTripper scenarios
func TestSSLIgnoreHandling(t *testing.T) {
	tests := []struct {
		name        string
		transport   http.RoundTripper
		expectPanic bool
	}{
		{
			name:        "Direct http.Transport should work",
			transport:   &http.Transport{},
			expectPanic: false,
		},
		{
			name:        "SpinnerRoundTripper with http.Transport should work",
			transport:   &apiclient.SpinnerRoundTripper{Next: &http.Transport{}},
			expectPanic: false,
		},
		{
			name:        "SpinnerRoundTripper with default transport should work",
			transport:   apiclient.NewSpinnerRoundTripper(),
			expectPanic: false,
		},
		{
			name:        "nil transport should work",
			transport:   nil,
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: tt.transport}
			
			// This simulates the SSL ignore logic from loginRun function
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()
			
			// Apply the SSL ignore logic (extracted from loginRun)
			applySSLIgnoreLogic(client)
			
			// Verify the SSL configuration was applied correctly
			verifySSLConfig(t, client)
		})
	}
}

// applySSLIgnoreLogic contains the same logic as in loginRun for handling SSL
func applySSLIgnoreLogic(httpClient *http.Client) {
	if httpClient.Transport == nil {
		httpClient.Transport = &http.Transport{}
	}

	// Handle both direct http.Transport and SpinnerRoundTripper wrapping http.Transport
	switch transport := httpClient.Transport.(type) {
	case *http.Transport:
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	case *apiclient.SpinnerRoundTripper:
		// If the SpinnerRoundTripper's Next is an http.Transport, configure it
		if httpTransport, ok := transport.Next.(*http.Transport); ok {
			httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			// If Next is not an http.Transport, replace it with one that has SSL verification disabled
			transport.Next = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	default:
		// Fallback: replace the transport entirely with one that ignores SSL errors
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
}

// verifySSLConfig checks that the SSL configuration was applied correctly
func verifySSLConfig(t *testing.T, httpClient *http.Client) {
	assert.NotNil(t, httpClient.Transport, "Transport should not be nil")
	
	switch transport := httpClient.Transport.(type) {
	case *http.Transport:
		assert.NotNil(t, transport.TLSClientConfig, "TLS config should be set")
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "InsecureSkipVerify should be true")
		
	case *apiclient.SpinnerRoundTripper:
		assert.NotNil(t, transport.Next, "SpinnerRoundTripper.Next should not be nil")
		
		if httpTransport, ok := transport.Next.(*http.Transport); ok {
			assert.NotNil(t, httpTransport.TLSClientConfig, "Underlying TLS config should be set")
			assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify, "Underlying InsecureSkipVerify should be true")
		} else {
			t.Errorf("SpinnerRoundTripper.Next should be *http.Transport, got %T", transport.Next)
		}
		
	default:
		t.Errorf("Unexpected transport type: %T", transport)
	}
}