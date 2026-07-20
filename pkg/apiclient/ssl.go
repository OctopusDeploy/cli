package apiclient

import (
	"crypto/tls"
	"net/http"
)

// ApplySSLIgnoreConfiguration configures the HTTP client to ignore SSL errors
// by setting InsecureSkipVerify on the underlying transport. This function
// handles multiple transport types:
// - Direct *http.Transport
// - *SpinnerRoundTripper wrapping *http.Transport
// - Any other transport type (fallback replacement)
func ApplySSLIgnoreConfiguration(httpClient *http.Client) {
	if httpClient.Transport == nil {
		httpClient.Transport = &http.Transport{}
	}

	// Handle both direct http.Transport and SpinnerRoundTripper wrapping http.Transport
	switch transport := httpClient.Transport.(type) {
	case *http.Transport:
		if transport.TLSClientConfig != nil {
			transport.TLSClientConfig.InsecureSkipVerify = true
		} else {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
	case *SpinnerRoundTripper:
		// If the SpinnerRoundTripper's Next is an http.Transport, configure it
		if httpTransport, ok := transport.Next.(*http.Transport); ok {
			if httpTransport.TLSClientConfig != nil {
				httpTransport.TLSClientConfig.InsecureSkipVerify = true
			} else {
				httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			}
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