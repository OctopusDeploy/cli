// Package agent holds what the installers need to know about the Octopus
// Kubernetes agent as it exists in a cluster: which agents are already
// installed, and which of the components they cooperate with are present.
package agent

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
)

// ChartName is the chart's own name, which is how an installed release is
// recognised whatever it was named.
const ChartName = "kubernetes-agent"

// ChartRef floats within the newest chart major version this tooling is known
// to work with, as the Octopus portal's generated command does. Bump the major
// together with KubernetesAgentUpgradeManager.LatestSupportedMajorVersion in
// Octopus Server.
var ChartRef = helm.ChartRef{Ref: "oci://registry-1.docker.io/octopusdeploy/kubernetes-agent", Version: "3.*.*"}

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
	return InstallationsFromReleases(runner, releases), nil
}

// InstallationsFromReleases picks the agents out of an already-fetched release
// list, for a caller that lists every release anyway: Helm reads a Secret per
// release to build the list, so it is worth listing once.
func InstallationsFromReleases(runner *helm.Runner, releases []helm.Release) []Installation {
	var installations []Installation
	for _, release := range releases {
		if release.Chart != ChartName {
			continue
		}
		installation := Installation{Release: release, Mode: ModeUnknown, ScriptPodClusterRole: true}

		values, err := runner.GetValues(release.Name, release.Namespace)
		if err == nil {
			installation.Name = helm.StringAt(values, "agent", "name")
			installation.Mode = modeFrom(values)
			installation.ScriptPodClusterRole = clusterRoleEnabled(values)
		}
		if installation.Name == "" {
			installation.Name = release.Name
		}

		installations = append(installations, installation)
	}
	return installations
}

func modeFrom(values map[string]any) Mode {
	switch {
	case helm.BoolAt(values, false, "agent", "worker", "enabled"):
		return ModeWorker
	case helm.BoolAt(values, false, "agent", "deploymentTarget", "enabled"):
		return ModeDeploymentTarget
	default:
		return ModeUnknown
	}
}

// clusterRoleEnabled defaults to true because that is the chart's own default,
// so a release that never set it has the cluster-wide permissions.
func clusterRoleEnabled(values map[string]any) bool {
	return helm.BoolAt(values, true, "scriptPods", "serviceAccount", "clusterRole", "enabled")
}

// supportedArchitectures are the node architectures the agent's images are
// built for. Anything else schedules and then crash-loops.
var supportedArchitectures = []string{"amd64", "arm64"}

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
	for _, arch := range supportedArchitectures {
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
