package kubernetes

import (
	"fmt"
	"net/url"
	"strings"
)

const DefaultOctopusGRPCPort = 8443

// DeriveGRPCURL is a starting point to confirm rather than a guarantee: a load
// balancer or proxy in front of Octopus often forwards only HTTPS. The
// connectivity preflight is what proves it.
func DeriveGRPCURL(serverURL string) string {
	if serverURL == "" {
		return ""
	}

	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}

	return fmt.Sprintf("grpc://%s:%d", parsed.Hostname(), DefaultOctopusGRPCPort)
}

// DefaultPollingPort is the TCP port Octopus Server listens on for polling
// Tentacles, which is how the Kubernetes agent connects.
const DefaultPollingPort = 10943

// cloudDomains are the Octopus Cloud domains. Cloud instances serve polling
// Tentacles on a separate hostname over 443, because the cluster's egress often
// only allows that port.
var cloudDomains = []string{".octopus.app", ".testoctopus.app"}

// IsOctopusCloud reports whether this server is hosted by Octopus, which serves
// every polling connection on one shared address however many nodes it runs.
func IsOctopusCloud(serverURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, domain := range cloudDomains {
		if strings.HasSuffix(host, domain) {
			return true
		}
	}
	return false
}

// DerivePollingURL is a starting point to confirm rather than a guarantee. The
// port is configurable on a self-hosted server, and a load balancer in front of
// Octopus has to pass the connection through untouched - the protocol needs an
// intact end-to-end TLS connection, so SSL offloading does not work. The
// connectivity preflight is what proves it.
func DerivePollingURL(serverURL string) string {
	if serverURL == "" {
		return ""
	}

	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}

	if IsOctopusCloud(serverURL) {
		return fmt.Sprintf("https://polling.%s", parsed.Hostname())
	}
	return fmt.Sprintf("https://%s:%d", parsed.Hostname(), DefaultPollingPort)
}
