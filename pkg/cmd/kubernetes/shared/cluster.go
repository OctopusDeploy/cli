// Package shared holds the parts of the Kubernetes installers every component
// needs: reaching a cluster, proving it can reach Octopus from the inside, and
// showing what is about to be installed before anything is created.
package shared

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
)

// Session is a live connection to a cluster, and what was learned while
// opening it.
type Session struct {
	Cluster *octoK8s.Cluster
	Runner  *helm.Runner
	Context octoK8s.Context
}

// Connector chooses a cluster, connects to it, and runs the component's own
// discovery against it.
type Connector struct {
	*cmd.Dependencies
	*octoK8s.CommonFlags

	// SelectMessage is asked when the kubeconfig holds more than one context.
	SelectMessage string
	// Discover is everything else the component reads from the cluster. It runs
	// inside the retry loop, so a credential problem anywhere in it can be fixed
	// and retried as one unit.
	Discover func(ctx context.Context, session *Session) error
	// Unrecoverable reports an error that neither retrying nor moving to another
	// cluster can fix, so neither is offered.
	Unrecoverable func(err error, kubeConfig *octoK8s.KubeConfig) bool
}

func (c *Connector) Connect(ctx context.Context) (*Session, error) {
	kubeConfig, err := octoK8s.LoadKubeConfig(c.KubeConfig.Value)
	if err != nil {
		return nil, err
	}

	for {
		session, err := c.connectAndDiscover(ctx, kubeConfig)
		if err == nil {
			return session, nil
		}

		retry, retryErr := c.ConfirmRetry(kubeConfig, err)
		if retryErr != nil {
			return nil, retryErr
		}
		if !retry {
			return nil, err
		}
	}
}

// connectAndDiscover holds everything that talks to the cluster, so a
// credential problem can be fixed and retried as a unit.
func (c *Connector) connectAndDiscover(ctx context.Context, kubeConfig *octoK8s.KubeConfig) (*Session, error) {
	if err := c.ResolveKubeContext(kubeConfig); err != nil {
		return nil, err
	}

	kubeContext, err := kubeConfig.FindContext(c.KubeContext.Value)
	if err != nil {
		return nil, err
	}

	cluster, err := octoK8s.Connect(kubeConfig, c.KubeContext.Value)
	if err != nil {
		return nil, err
	}

	// Building a client is offline, so this is the first call that proves the
	// credentials work. Cloud clusters authenticate through a helper such as
	// gcloud or aws, which fails here when its session has expired.
	version, err := cluster.ServerVersion()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(c.Out, "Connected to %s %s\n", output.Cyan(c.KubeContext.Value), output.Dimf("(Kubernetes %s)", version))

	runner, err := helm.NewRunner(c.KubeConfig.Value, c.KubeContext.Value, c.Out)
	if err != nil {
		return nil, err
	}

	session := &Session{Cluster: cluster, Runner: runner, Context: kubeContext}
	if c.Discover == nil {
		return session, nil
	}
	if err := c.Discover(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// ResolveKubeContext always reports the chosen context rather than silently
// assuming one: installing into the wrong cluster is the most expensive mistake
// available here.
func (c *Connector) ResolveKubeContext(kubeConfig *octoK8s.KubeConfig) error {
	contexts := kubeConfig.Contexts()
	if len(contexts) == 0 {
		return errors.New("your kubeconfig does not contain any contexts")
	}

	if c.KubeContext.Value != "" {
		if _, err := kubeConfig.FindContext(c.KubeContext.Value); err != nil {
			return err
		}
		return nil
	}

	current, hasCurrent := kubeConfig.CurrentContext()
	if c.NoPrompt {
		if !hasCurrent {
			return fmt.Errorf("your kubeconfig has no current context, so --%s must be specified", octoK8s.FlagKubeContext)
		}
		c.KubeContext.Value = current.Name
		return nil
	}

	if len(contexts) == 1 {
		c.KubeContext.Value = contexts[0].Name
		return nil
	}

	selected, err := selectors.Select(c.Ask, c.selectMessage(),
		func() ([]octoK8s.Context, error) { return contexts, nil },
		func(ctx octoK8s.Context) string { return ctx.Display() })
	if err != nil {
		return err
	}
	c.KubeContext.Value = selected.Name
	return nil
}

func (c *Connector) selectMessage() string {
	if c.SelectMessage == "" {
		return "Which cluster should this be installed into?"
	}
	return c.SelectMessage
}

// ConfirmRetry avoids ending the command and discarding everything already
// answered. Expired cloud credentials are the common case, and are usually
// fixed in another terminal in seconds.
func (c *Connector) ConfirmRetry(kubeConfig *octoK8s.KubeConfig, cause error) (bool, error) {
	if c.NoPrompt {
		return false, nil
	}
	if c.Unrecoverable != nil && c.Unrecoverable(cause, kubeConfig) {
		return false, nil
	}

	fmt.Fprintf(c.Out, "\n%s %v\n", output.Red("✘"), cause)

	const (
		tryAgain  = "Try again"
		pickOther = "Choose a different cluster"
		cancel    = "Cancel"
	)

	choices := []string{tryAgain}
	if len(kubeConfig.Contexts()) > 1 {
		choices = append(choices, pickOther)
	}
	choices = append(choices, cancel)

	answer := ""
	if err := c.Ask(&survey.Select{
		Message: "What would you like to do?",
		Options: choices,
		Help:    "If a cloud credential helper failed, sign in again in another terminal and choose Try again.",
	}, &answer); err != nil {
		return false, err
	}

	switch answer {
	case tryAgain:
		return true, nil
	case pickOther:
		// Sends ResolveKubeContext back to the prompt.
		c.KubeContext.Value = ""
		return true, nil
	default:
		return false, nil
	}
}
