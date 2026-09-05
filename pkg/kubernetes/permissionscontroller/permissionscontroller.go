// Package permissionscontroller holds what the installers need to know about
// the Octopus permissions controller as it exists in a cluster.
package permissionscontroller

import (
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
)

// ChartName is the chart's own name, which is how an installed release is
// recognised whatever it was named.
const ChartName = "octopus-permissions-controller-chart"

var ChartRef = helm.ChartRef{Ref: "oci://registry-1.docker.io/octopusdeploy/octopus-permissions-controller-chart"}

const (
	// APIGroup is shared with the agent's own script pod templates, so presence
	// is checked by resource rather than by group.
	APIGroup                       = "agent.octopus.com"
	workloadServiceAccountResource = "workloadserviceaccounts"

	certManagerAPIGroup = "cert-manager.io"
	certificateResource = "certificates"
)

// Present reports whether the controller is running in this cluster. It is
// found by its own custom resource rather than by a Helm release, which can be
// named anything.
func Present(cluster *octoK8s.Cluster) (bool, error) {
	return cluster.HasAPIResource(APIGroup, workloadServiceAccountResource)
}

// CertManagerPresent reports whether cert-manager is installed, which the
// controller needs for its admission webhook's certificate.
func CertManagerPresent(cluster *octoK8s.Cluster) (bool, error) {
	return cluster.HasAPIResource(certManagerAPIGroup, certificateResource)
}
