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
