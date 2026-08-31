// Package agent holds what the installers need to know about the Octopus
// Kubernetes agent as it exists in a cluster: which agents are already
// installed, and which of the components they cooperate with are present.
package agent

import (
	"fmt"
	"strings"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
)

// ChartName is the chart's own name, which is how an installed release is
// recognised whatever it was named.
const ChartName = "kubernetes-agent"

// Mode is what an agent was registered as. One agent is either a deployment
// target or a worker, never both: the chart's registration takes one path or
// the other.
type Mode string

const (
	ModeDeploymentTarget Mode = "deployment target"
	ModeWorker           Mode = "worker"
	// ModeUnknown covers a release whose values could not be read, which a
	// namespace-scoped credential is entitled to be unable to do.
	ModeUnknown Mode = "unknown"
)

// Installation is an agent already installed in this cluster.
type Installation struct {
	Release helm.Release
	// Name is the name the agent registered with, which is not necessarily the
	// Helm release name.
	Name string
	Mode Mode
	// ScriptPodClusterRole reports whether the agent's script pods still hold
	// the chart's default cluster-wide permissions.
	ScriptPodClusterRole bool
}

// Installations lists the agents in the cluster, so an installer can say when a
// name is already taken and what an existing agent would need to change.
func Installations(runner *helm.Runner) ([]Installation, error) {
	releases, err := runner.FindByChart(ChartName)
	if err != nil {
		return nil, err
	}

	installations := make([]Installation, 0, len(releases))
	for _, release := range releases {
		installation := Installation{Release: release, Mode: ModeUnknown, ScriptPodClusterRole: true}

		values, err := runner.GetValues(release.Name, release.Namespace)
		if err == nil {
			installation.Name = stringAt(values, "agent", "name")
			installation.Mode = modeFrom(values)
			installation.ScriptPodClusterRole = clusterRoleEnabled(values)
		}
		if installation.Name == "" {
			installation.Name = release.Name
		}

		installations = append(installations, installation)
	}
	return installations, nil
}

func modeFrom(values map[string]any) Mode {
	switch {
	case boolAt(values, false, "agent", "worker", "enabled"):
		return ModeWorker
	case boolAt(values, false, "agent", "deploymentTarget", "enabled"):
		return ModeDeploymentTarget
	default:
		return ModeUnknown
	}
}

// clusterRoleEnabled defaults to true because that is the chart's own default,
// so a release that never set it has the cluster-wide permissions.
func clusterRoleEnabled(values map[string]any) bool {
	return boolAt(values, true, "scriptPods", "serviceAccount", "clusterRole", "enabled")
}

// PermissionsControllerPresent reports whether the Octopus permissions
// controller is running in this cluster. It is found by its own custom
// resource rather than by a Helm release, which can be named anything.
func PermissionsControllerPresent(cluster *octoK8s.Cluster) (bool, error) {
	return cluster.HasAPIResource(PermissionsControllerAPIGroup, workloadServiceAccountResource)
}

// CertManagerPresent reports whether cert-manager is installed, which the
// permissions controller needs for its admission webhook's certificate.
func CertManagerPresent(cluster *octoK8s.Cluster) (bool, error) {
	return cluster.HasAPIResource(certManagerAPIGroup, certificateResource)
}

const (
	// PermissionsControllerAPIGroup is shared with the agent's own script pod
	// templates, so presence is checked by resource rather than by group.
	PermissionsControllerAPIGroup  = "agent.octopus.com"
	workloadServiceAccountResource = "workloadserviceaccounts"

	certManagerAPIGroup = "cert-manager.io"
	certificateResource = "certificates"
)

// SupportedArchitectures are the node architectures the agent's images are
// built for. Anything else schedules and then crash-loops.
var SupportedArchitectures = []string{"amd64", "arm64"}

// ErrUnsupportedNodes means no node in this cluster can run the agent, which is
// a property of the cluster rather than of anything the caller did.
type ErrUnsupportedNodes struct {
	Architectures []string
}

func (e ErrUnsupportedNodes) Error() string {
	return fmt.Sprintf("the Octopus agent only runs on linux/amd64 and linux/arm64 nodes, and this cluster has none (its nodes are %s)",
		strings.Join(e.Architectures, ", "))
}

// UnsupportedArchitectures returns the architectures in a cluster the agent
// cannot run on. An empty result covers both a supported cluster and one whose
// nodes could not be listed.
func UnsupportedArchitectures(present []string) []string {
	supported := map[string]bool{}
	for _, arch := range SupportedArchitectures {
		supported[arch] = true
	}

	var unsupported []string
	for _, arch := range present {
		if !supported[arch] {
			unsupported = append(unsupported, arch)
		}
	}
	return unsupported
}

// RunnableArchitecture reports whether any node in the cluster can run the
// agent. A cluster whose nodes could not be listed is assumed to be fine.
func RunnableArchitecture(present []string) bool {
	if len(present) == 0 {
		return true
	}
	return len(UnsupportedArchitectures(present)) < len(present)
}

func stringAt(values map[string]any, keys ...string) string {
	value, ok := at(values, keys...).(string)
	if !ok {
		return ""
	}
	return value
}

func boolAt(values map[string]any, fallback bool, keys ...string) bool {
	value, ok := at(values, keys...).(bool)
	if !ok {
		return fallback
	}
	return value
}

// at walks a Helm values tree, which only holds what was set explicitly, so any
// step of the path may be missing.
func at(values map[string]any, keys ...string) any {
	var current any = values
	for _, key := range keys {
		node, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = node[key]
		if !ok {
			return nil
		}
	}
	return current
}
