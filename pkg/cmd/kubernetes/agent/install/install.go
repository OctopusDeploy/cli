package install

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/accesstokens"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/octopusservernodes"
	"github.com/OctopusDeploy/cli/pkg/output"
)

// The agent registers itself, so an Octopus credential has to reach the
// cluster. Passing it by Secret reference keeps it out of the Helm release
// values, out of any file written by --output-values, and out of the process
// table. The key name is the chart's contract.
const (
	tokenSecretName = "octopus-agent-registration-token"
	tokenSecretKey  = "bearer-token"
)

type InstallOptions struct {
	*InstallFlags
	*cmd.Dependencies

	// Mode decides what the agent registers as, which questions are asked, and
	// which half of the chart's values are filled in. One agent is either a
	// deployment target or a worker; the chart registers it one way or the other.
	Mode agentK8s.Mode

	*sharedTarget.CreateTargetEnvironmentOptions
	*machinescommon.CreateTargetMachinePolicyOptions
	*sharedWorker.WorkerPoolOptions

	// AccessTokenCallback gets the credential the agent registers itself with.
	// Injected so the flow can be tested without a server.
	AccessTokenCallback func() (accesstokens.Token, error)
	// RegisteredCallback reports whether Octopus already has a deployment target
	// or worker of this name, which is how a clash is caught before installing
	// and an orphan is caught after a failure.
	RegisteredCallback func(name string) (bool, error)
	// TargetTagsCallback lists the target tags the space already knows about.
	TargetTagsCallback func() ([]string, error)
	// ServerNodesCallback lists the Octopus Server's own task-running nodes,
	// which is how a High Availability cluster is recognised: the agent polls
	// every node, and each needs its own address.
	ServerNodesCallback func() ([]octopusservernodes.Node, error)

	// Populated by Discover before prompting. Exported so tests can drive the
	// prompt flow against a fake cluster.
	Cluster           *octoK8s.Cluster
	Runner            *helm.Runner
	KubeContextInfo   octoK8s.Context
	StorageClasses    []octoK8s.StorageClass
	NodeArchitectures []string
	// KnownTargetTags is what the space already had, so the review can say which
	// of the chosen tags are new.
	KnownTargetTags       []string
	Installations         []agentK8s.Installation
	PermissionsController bool
	// ScriptPodRules are the rules copied from ScriptPodRole, in the plain shape
	// a Helm value has to be.
	ScriptPodRules []any

	// AccessModeChosen records that --read-write-many was given explicitly, so
	// the storage class does not get to decide.
	AccessModeChosen bool

	TargetNamespace string
	TargetRelease   string
	Token           accesstokens.Token

	// registeredBefore is the answer to RegisteredCallback for
	// registrationCheckedFor, kept so the same question is not asked of Octopus
	// three times in one run.
	registeredBefore       bool
	registrationCheckedFor string

	// serverNodes is the answer from ServerNodesCallback, read once and kept.
	serverNodes     []octopusservernodes.Node
	serverNodesRead bool
}

// alreadyRegistered answers whether Octopus already has an agent of this name.
// It is asked before installing, to warn that a name is taken, and again after
// a failure, to tell an orphaned registration from one that was always there.
func (opts *InstallOptions) alreadyRegistered() (bool, error) {
	if opts.registrationCheckedFor == opts.Name.Value {
		return opts.registeredBefore, nil
	}
	if opts.RegisteredCallback == nil {
		return false, nil
	}

	taken, err := opts.RegisteredCallback(opts.Name.Value)
	if err != nil {
		return false, err
	}
	opts.registeredBefore = taken
	opts.registrationCheckedFor = opts.Name.Value
	return taken, nil
}

func NewInstallOptions(installFlags *InstallFlags, dependencies *cmd.Dependencies, mode agentK8s.Mode) *InstallOptions {
	return &InstallOptions{
		InstallFlags: installFlags,
		Dependencies: dependencies,
		Mode:         mode,

		CreateTargetEnvironmentOptions:   sharedTarget.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetMachinePolicyOptions: machinescommon.NewCreateTargetMachinePolicyOptions(dependencies),
		WorkerPoolOptions:                sharedWorker.NewWorkerPoolOptions(dependencies),

		AccessTokenCallback: func() (accesstokens.Token, error) {
			return accesstokens.Generate(dependencies.Client)
		},
		RegisteredCallback: func(name string) (bool, error) {
			return registered(dependencies, mode, name)
		},
		TargetTagsCallback: func() ([]string, error) {
			return sharedTarget.TargetTagNames(dependencies.Client)
		},
		ServerNodesCallback: func() ([]octopusservernodes.Node, error) {
			return octopusservernodes.TaskNodes(dependencies.Client)
		},
	}
}

func installRun(ctx context.Context, opts *InstallOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := opts.Discover(ctx); err != nil {
		return err
	}

	if opts.NoPrompt {
		if err := opts.ValidateForAutomation(); err != nil {
			return err
		}
		if err := opts.ResolveWithoutPrompting(ctx); err != nil {
			return err
		}
	} else {
		if err := PromptMissing(ctx, opts); err != nil {
			return err
		}
		// Most of this was worked out rather than asked for, so show all of it
		// before anything is created.
		if err := Confirm(ctx, opts); err != nil {
			return err
		}
	}

	return opts.Commit(ctx)
}

// Run installs using an existing set of dependencies. The `kubernetes install`
// wizard uses this to hand off after the user picks a component, so the two
// entry points share one implementation.
func Run(ctx context.Context, dependencies *cmd.Dependencies) error {
	return installRun(ctx, NewInstallOptions(NewInstallFlags(), dependencies, agentK8s.ModeDeploymentTarget))
}

func RunWorker(ctx context.Context, dependencies *cmd.Dependencies) error {
	return installRun(ctx, NewInstallOptions(NewInstallFlags(), dependencies, agentK8s.ModeWorker))
}

func (opts *InstallOptions) chartRef() helm.ChartRef {
	return agentK8s.ChartRef.WithVersion(opts.ChartVersion.Value)
}

var errEulaDeclined = errors.New("the Octopus Customer Agreement has to be accepted to install the agent")

func (opts *InstallOptions) reportUnsupportedNodes() {
	unsupported := agentK8s.UnsupportedArchitectures(opts.NodeArchitectures)
	if len(unsupported) == 0 {
		return
	}
	fmt.Fprintf(opts.Out, "%s This cluster has %s nodes, which the agent cannot run on. It will only schedule on the linux/amd64 and linux/arm64 nodes.\n",
		output.Yellow("!"), strings.Join(unsupported, " and "))
}
