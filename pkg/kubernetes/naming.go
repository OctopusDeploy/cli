package kubernetes

import (
	"fmt"
	"regexp"
	"strings"
)

// Namespaces match what the Octopus portal generates, so a CLI install and a
// portal install of the same name land in the same place.
const (
	ArgoCDGatewayNamespacePrefix = "octo-argo-gateway-"
	AgentNamespacePrefix         = "octopus-agent-"
	WorkerNamespacePrefix        = "octopus-worker-"

	// Only one permissions controller can run per cluster, so this is fixed.
	PermissionsControllerNamespace = "octopus-permissions-controller-system"
)

const (
	dnsLabelMaxLen    = 63 // Kubernetes limit for a namespace name (RFC 1123 label)
	releaseNameMaxLen = 53 // Helm's own limit on release names
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(name string) (string, error) {
	s := nonSlugChars.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "", fmt.Errorf("%q does not contain any letters or digits, so a Kubernetes name cannot be derived from it", name)
	}
	return s, nil
}

func ReleaseName(name string) (string, error) {
	s, err := Slug(name)
	if err != nil {
		return "", err
	}
	return truncateSlug(s, releaseNameMaxLen), nil
}

// DerivedNamespace is derived rather than prompted for because most people
// install each component into its own new namespace. --namespace overrides it.
func DerivedNamespace(prefix, name string) (string, error) {
	if len(prefix) >= dnsLabelMaxLen {
		return "", fmt.Errorf("namespace prefix %q is too long", prefix)
	}
	s, err := Slug(name)
	if err != nil {
		return "", err
	}
	return prefix + truncateSlug(s, dnsLabelMaxLen-len(prefix)), nil
}

// ResolveNames keeps an explicit --namespace or --release-name and derives
// whatever was not given from the component's name.
func ResolveNames(explicitNamespace, explicitRelease, prefix, name string) (namespace, release string, err error) {
	namespace = explicitNamespace
	if namespace == "" {
		if namespace, err = DerivedNamespace(prefix, name); err != nil {
			return "", "", err
		}
	}

	release = explicitRelease
	if release == "" {
		if release, err = ReleaseName(name); err != nil {
			return "", "", err
		}
	}
	return namespace, release, nil
}

// truncateSlug cuts on a hyphen boundary where it can, to stay readable.
func truncateSlug(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	if i := strings.LastIndex(s, "-"); i > max/2 {
		s = s[:i]
	}
	return strings.TrimRight(s, "-")
}
