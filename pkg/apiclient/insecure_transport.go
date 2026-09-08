package apiclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

// DisableTLSVerification turns off TLS certificate verification for the given
// client, for callers such as `login --ignore-ssl-errors`.
//
// The client's transport is not necessarily a plain *http.Transport:
// NewClientFactoryFromConfig wraps it in a SpinnerRoundTripper, so asserting
// the type panics. Unwrap whatever round-trippers were layered on to reach the
// real transport, and swap in an insecure clone rather than editing it in
// place, since that transport is usually http.DefaultTransport and is shared
// with every other client in the process.
func DisableTLSVerification(client *http.Client) error {
	transport, err := insecureTransport(client.Transport)
	if err != nil {
		return err
	}

	client.Transport = transport
	return nil
}

func insecureTransport(roundTripper http.RoundTripper) (http.RoundTripper, error) {
	switch transport := roundTripper.(type) {
	case nil:
		// http.Client falls back to http.DefaultTransport when it has none of
		// its own, so a fresh transport is equivalent bar the TLS config.
		return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, nil

	case *SpinnerRoundTripper:
		next, err := insecureTransport(transport.Next)
		if err != nil {
			return nil, err
		}
		transport.Next = next
		return transport, nil

	case *http.Transport:
		insecure := transport.Clone()
		if insecure.TLSClientConfig == nil {
			insecure.TLSClientConfig = &tls.Config{}
		}
		insecure.TLSClientConfig.InsecureSkipVerify = true
		return insecure, nil

	default:
		// Better to say so than to quietly leave verification on and report a
		// certificate error the caller has already asked us to ignore.
		return nil, fmt.Errorf("cannot ignore SSL errors: unsupported HTTP transport %T", roundTripper)
	}
}
