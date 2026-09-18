package install

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
)

func shortDescription(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return "Install the Octopus Kubernetes agent as a worker"
	}
	return "Install the Octopus Kubernetes agent as a deployment target"
}

func longDescription(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return heredoc.Doc(`
			Install the Octopus Kubernetes agent into a Kubernetes cluster as a worker.

			The worker runs Octopus steps in the cluster, one pod per task, and releases the
			compute again when the task finishes. It polls Octopus for work, so the cluster does
			not need to be reachable from outside.

			Run without arguments to be prompted. Anything that can be read from the cluster or
			from Octopus is filled in for you: the Octopus server, space and polling address, the
			cluster's storage classes, and the install namespace.
		`)
	}

	return heredoc.Doc(`
		Install the Octopus Kubernetes agent into a Kubernetes cluster as a deployment target.

		The agent runs Kubernetes steps from inside the cluster, so Octopus does not need
		cluster credentials and the cluster does not need to be reachable from outside - the
		agent polls Octopus for work.

		Run without arguments to be prompted. Anything that can be read from the cluster or
		from Octopus is filled in for you: the Octopus server, space and polling address, the
		cluster's storage classes, and the install namespace.
	`)
}

func examples(mode agentK8s.Mode) string {
	if mode == agentK8s.ModeWorker {
		return heredoc.Docf(`
			$ %[1]s kubernetes worker install
			$ %[1]s kubernetes worker install --name cluster-worker --worker-pool "Kubernetes Pool" --dry-run
			$ %[1]s kubernetes worker install --name cluster-worker --worker-pool "Kubernetes Pool" --accept-eula --no-prompt
		`, constants.ExecutableName)
	}

	return heredoc.Docf(`
		$ %[1]s kubernetes agent install
		$ %[1]s kubernetes agent install --name production --environment Production --role k8s --dry-run
		$ %[1]s kubernetes agent install --name production --environment Production --role k8s --accept-eula --no-prompt
	`, constants.ExecutableName)
}
