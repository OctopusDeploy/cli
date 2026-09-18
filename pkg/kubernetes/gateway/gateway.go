// Package gateway holds what the installers and operational commands need to
// know about the Octopus Argo CD gateway as it exists in a cluster: its chart,
// its workload, and the Secret contract its chart reads credentials from.
package gateway

import (
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
)

// ChartName is the chart's own name, which is how an installed release is
// recognised whatever it was named.
const ChartName = "octopus-argocd-gateway-chart"

var ChartRef = helm.ChartRef{Ref: "oci://registry-1.docker.io/octopusdeploy/octopus-argocd-gateway-chart"}

// DeploymentSelector matches the gateway deployment the chart installs.
const DeploymentSelector = "app.kubernetes.io/name=octopus-argocd-gateway"

// Passing credentials by Secret reference keeps them out of the Helm release
// values, out of any file written by --output-values, and out of the process
// table. These names and keys are the chart's contract.
const (
	ArgoTokenSecretName    = "octopus-argocd-gateway-argocd-token"
	ArgoTokenSecretKey     = "ARGOCD_AUTH_TOKEN"
	ProjectTokenSecretName = "octopus-argocd-gateway-project-tokens"

	// The chart reads project tokens from Secret keys of this shape, exposed as
	// environment variables with an OCTOPUS_ARGOCD_ prefix added by envFrom.
	ProjectTokenKeyPrefix = "PROJECT_AUTH_TOKEN_"
	AccountTokenEnvName   = "OCTOPUS_ARGOCD_AUTH_TOKEN"

	// The chart's own registration job would write these, and the gateway
	// reads them from its projected configuration volume. Octopus writes them
	// instead, so no Octopus credential of the user's ever enters the cluster.
	RegistrationSecretName = "octopus-argocd-gateway-octopus-auth-secret"
	RegistrationSecretKey  = "octopus-argocd-gateway-octopus-authentication-secret.yaml"
)

// InstanceFromValues reads the gateway's Argo CD connection back out of a
// release's values, so a gateway can be operated on without knowing how it was
// installed.
func InstanceFromValues(values map[string]any) argocd.Instance {
	return argocd.Instance{
		ServerGRPCURL: helm.StringAt(values, "gateway", "argocd", "serverGrpcUrl"),
		WebUIURL:      helm.StringAt(values, "registration", "argocd", "webUiUrl"),
	}
}
