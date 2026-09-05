package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/permissionscontroller"
)

func (opts *InstallOptions) Discover(ctx context.Context) error {
	connector := opts.connector()
	session, err := connector.Connect(ctx)
	if err != nil {
		return err
	}

	opts.Cluster = session.Cluster
	opts.Runner = session.Runner
	opts.KubeContextInfo = session.Context
	return nil
}

func (opts *InstallOptions) connector() *shared.Connector {
	return &shared.Connector{
		Dependencies:  opts.Dependencies,
		CommonFlags:   opts.CommonFlags,
		SelectMessage: fmt.Sprintf("Which cluster should the %s be installed into?", opts.installedThing()),
		Discover:      opts.discoverCluster,
		Unrecoverable: func(cause error, kubeConfig *octoK8s.KubeConfig) bool {
			// Nothing to retry, and no other cluster to move to.
			return errors.As(cause, &agentK8s.ErrUnsupportedNodes{}) && len(kubeConfig.Contexts()) == 1
		},
	}
}

// discoverCluster reads everything the questions and the review screen are
// filled in from, so a credential problem anywhere in it can be fixed and
// retried as one unit.
func (opts *InstallOptions) discoverCluster(ctx context.Context, session *shared.Session) error {
	architectures, err := session.Cluster.NodeArchitectures(ctx)
	if err != nil {
		return err
	}
	opts.NodeArchitectures = architectures

	if !agentK8s.RunnableArchitecture(architectures) {
		return agentK8s.ErrUnsupportedNodes{Architectures: architectures}
	}

	classes, err := session.Cluster.StorageClasses(ctx)
	if err != nil {
		return err
	}
	opts.StorageClasses = classes

	installations, err := agentK8s.Installations(session.Runner)
	if err != nil {
		return err
	}
	opts.Installations = installations

	present, err := permissionscontroller.Present(session.Cluster)
	if err != nil {
		return err
	}
	opts.PermissionsController = present

	return nil
}
