package apiclient

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestApplySSLIgnoreConfiguration tests the ApplySSLIgnoreConfiguration function
func TestApplySSLIgnoreConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *http.Client
	}{
		{
			name: "nil transport",
			setupFunc: func() *http.Client {
				return &http.Client{}
			},
		},
		{
			name: "direct http.Transport with nil TLSClientConfig",
			setupFunc: func() *http.Client {
				return &http.Client{
					Transport: &http.Transport{},
				}
			},
		},
		{
			name: "direct http.Transport with existing TLSClientConfig",
			setupFunc: func() *http.Client {
				return &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							ServerName: "example.com",
						},
					},
				}
			},
		},
		{
			name: "SpinnerRoundTripper with nil TLSClientConfig",
			setupFunc: func() *http.Client {
				return &http.Client{
					Transport: &SpinnerRoundTripper{
						Next: &http.Transport{},
					},
				}
			},
		},
		{
			name: "SpinnerRoundTripper with existing TLSClientConfig",
			setupFunc: func() *http.Client {
				return &http.Client{
					Transport: &SpinnerRoundTripper{
						Next: &http.Transport{
							TLSClientConfig: &tls.Config{
								ServerName: "example.com",
							},
						},
					},
				}
			},
		},
		{
			name: "SpinnerRoundTripper with default transport",
			setupFunc: func() *http.Client {
				return &http.Client{
					Transport: NewSpinnerRoundTripper(),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupFunc()
			
			// Apply the SSL ignore configuration
			ApplySSLIgnoreConfiguration(client)
			
			// Verify the configuration was applied correctly
			verifySSLIgnored(t, client)
		})
	}
}

func verifySSLIgnored(t *testing.T, client *http.Client) {
	assert.NotNil(t, client.Transport, "Transport should not be nil")
	
	switch transport := client.Transport.(type) {
	case *http.Transport:
		assert.NotNil(t, transport.TLSClientConfig, "TLS config should be set")
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "InsecureSkipVerify should be true")
		
	case *SpinnerRoundTripper:
		assert.NotNil(t, transport.Next, "SpinnerRoundTripper.Next should not be nil")
		
		httpTransport, ok := transport.Next.(*http.Transport)
		assert.True(t, ok, "SpinnerRoundTripper.Next should be *http.Transport")
		assert.NotNil(t, httpTransport.TLSClientConfig, "Underlying TLS config should be set")
		assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify, "Underlying InsecureSkipVerify should be true")
		
	default:
		t.Errorf("Unexpected transport type: %T", transport)
	}
}