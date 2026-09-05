package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"k8s.io/client-go/rest"
)

// ConfigureAccountAndMintToken replaces the most tedious part of connecting
// Argo CD to Octopus: editing argocd-cm and argocd-rbac-cm, then running the
// Argo CD CLI. Callers fall back to the manual path if this returns an error.
func ConfigureAccountAndMintToken(ctx context.Context, opts *InstallOptions, status argocd.AccountStatus) (string, error) {
	if !status.IsComplete() {
		fmt.Fprintf(opts.Out, "Updating Argo CD configuration...\n")
		if err := argocd.ConfigureAccount(ctx, opts.Cluster, opts.Instance, status); err != nil {
			return "", err
		}
	}

	restConfig, err := opts.restConfig()
	if err != nil {
		return "", err
	}

	client, closePortForward, err := argocd.Dial(ctx, opts.Cluster, restConfig, opts.Instance)
	if err != nil {
		return "", err
	}
	defer closePortForward()

	revert, err := SignIn(ctx, opts.Out, client, loginStrategies(opts, status.Spec))
	if revert != nil {
		defer revert()
	}
	if err != nil {
		return "", err
	}

	// Argo CD reloads argocd-cm in the background.
	if err := client.WaitForAccount(ctx, status.Spec.Name); err != nil {
		return "", err
	}

	token, err := client.GenerateToken(ctx, status.Spec.Name)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(opts.Out, "%s Generated an Argo CD token for the %s account\n",
		output.Green("✔"), output.Cyan(status.Spec.Name))

	reportAccess(opts, client.VerifyAccess(ctx))
	return token, nil
}

// LoginStrategy is one way to obtain an Argo CD session. Each is only acted on
// when it is reached, because some of them change the cluster.
type LoginStrategy struct {
	Describe string
	Begin    func(context.Context) (argocd.Credentials, func(), error)
}

// ArgoLogin is the part of an Argo CD client SignIn needs.
type ArgoLogin interface {
	Login(ctx context.Context, credentials argocd.Credentials) error
}

// SignIn works through the ways of signing in until one is accepted.
//
// A rejection is not the end of it: the initial admin password is tried first
// because using it changes nothing, but Argo CD leaves that Secret in place
// when the admin password is changed, so a stale one is only discovered by
// being turned away.
func SignIn(ctx context.Context, out io.Writer, client ArgoLogin, strategies []LoginStrategy) (func(), error) {
	var attempts []string

	for _, strategy := range strategies {
		credentials, revert, err := strategy.Begin(ctx)
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", strategy.Describe, err))
			continue
		}

		if err := client.Login(ctx, credentials); err != nil {
			if revert != nil {
				revert()
			}
			attempts = append(attempts, fmt.Sprintf("%s: %v", strategy.Describe, err))
			fmt.Fprintf(out, "  %s %s did not work, trying another way\n", output.Yellow("!"), strategy.Describe)
			continue
		}

		return revert, nil
	}

	return nil, fmt.Errorf("could not sign in to Argo CD. Octopus tried:\n  %s", strings.Join(attempts, "\n  "))
}

func loginStrategies(opts *InstallOptions, spec argocd.AccountSpec) []LoginStrategy {
	namespace := opts.Instance.Namespace

	strategies := []LoginStrategy{{
		Describe: "the initial admin password",
		Begin: func(ctx context.Context) (argocd.Credentials, func(), error) {
			diagnosis, err := argocd.DiagnoseAuth(ctx, opts.Cluster, opts.Instance)
			if err != nil {
				return argocd.Credentials{}, nil, err
			}
			if !diagnosis.AdminEnabled || !diagnosis.HasInitialAdminSecret {
				return argocd.Credentials{}, nil, errors.New(diagnosis.Explain())
			}

			password, found, err := argocd.InitialAdminPassword(ctx, opts.Cluster, namespace)
			if err != nil || !found {
				return argocd.Credentials{}, nil, err
			}
			return argocd.Credentials{Username: argocd.AdminUsername, Password: password}, nil, nil
		},
	}, {
		Describe: fmt.Sprintf("a temporary password on the %s account", spec.Name),
		Begin: func(ctx context.Context) (argocd.Credentials, func(), error) {
			if !opts.NoPrompt {
				if err := confirmBootstrap(opts, spec); err != nil {
					return argocd.Credentials{}, nil, err
				}
			}

			bootstrap, err := argocd.BeginBootstrapLogin(ctx, opts.Cluster, opts.Instance, spec)
			if err != nil {
				return argocd.Credentials{}, nil, err
			}
			return bootstrap.Credentials(), revertBootstrap(opts, bootstrap, spec), nil
		},
	}}

	if opts.NoPrompt {
		return strategies
	}

	return append(strategies, LoginStrategy{
		Describe: "credentials you supply",
		Begin: func(context.Context) (argocd.Credentials, func(), error) {
			credentials, err := askForCredentials(opts)
			return credentials, nil, err
		},
	})
}

func revertBootstrap(opts *InstallOptions, bootstrap *argocd.BootstrapLogin, spec argocd.AccountSpec) func() {
	return func() {
		// A fresh context: the caller's may already be cancelled, and the
		// temporary password still has to go.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := bootstrap.Revert(ctx); err != nil {
			fmt.Fprintf(opts.Out, "%s Could not remove the temporary Argo CD password for %s: %v\n"+
				"  Remove accounts.%s.password from the %s/%s Secret by hand.\n",
				output.Yellow("!"), spec.Name, err, spec.Name, opts.Instance.Namespace, argocd.SecretName)
		}
	}
}

// confirmBootstrap asks first because argocd-secret is where Argo CD keeps its
// TLS and signing keys.
func confirmBootstrap(opts *InstallOptions, spec argocd.AccountSpec) error {
	fmt.Fprintf(opts.Out, "\nOctopus can still get a token without an administrator password, by giving the\n"+
		"%s account a temporary password of its own and removing it afterwards.\n", output.Cyan(spec.Name))
	fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
		"This sets accounts.%s.password in the %s/%s Secret. Nothing else in that Secret is touched, and the administrator's password is not read or changed.",
		spec.Name, opts.Instance.Namespace, argocd.SecretName))

	proceed := false
	if err := opts.Ask(&survey.Confirm{
		Message: "Generate the token this way?",
		Default: true,
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("declined")
	}
	return nil
}

func askForCredentials(opts *InstallOptions) (argocd.Credentials, error) {
	fmt.Fprintf(opts.Out, "\nSigning in to Argo CD needs an account that can create API keys.\n")

	credentials := argocd.Credentials{Username: argocd.AdminUsername}
	if err := opts.Ask(&survey.Input{
		Message: "Argo CD username",
		Default: argocd.AdminUsername,
	}, &credentials.Username, survey.WithValidator(survey.Required)); err != nil {
		return argocd.Credentials{}, err
	}

	if err := opts.Ask(&survey.Password{
		Message: fmt.Sprintf("Argo CD password for %s", credentials.Username),
	}, &credentials.Password, survey.WithValidator(survey.Required)); err != nil {
		return argocd.Credentials{}, err
	}

	return credentials, nil
}

func (opts *InstallOptions) restConfig() (*rest.Config, error) {
	kubeConfig, err := octoK8s.LoadKubeConfig(opts.KubeConfig.Value)
	if err != nil {
		return nil, err
	}
	return kubeConfig.RestConfig(opts.KubeContext.Value)
}

// reportAccess exists because Argo CD answers an under-privileged request with
// an empty list rather than a refusal, so a gateway can connect and then show
// nothing.
func reportAccess(opts *InstallOptions, access argocd.AccessCheck) {
	if !access.Readable() {
		fmt.Fprintf(opts.Out, "  %s The token could not read %s. Check the RBAC policies in %s.\n",
			output.Yellow("!"), unreadable(access), argocd.RBACConfigMapName)
		return
	}

	fmt.Fprintf(opts.Out, "  %s\n", output.Dimf("It can read %d %s and %d %s.",
		access.Applications, util.Pluralise("application", "applications", access.Applications),
		access.Clusters, util.Pluralise("cluster", "clusters", access.Clusters)))
}

func unreadable(access argocd.AccessCheck) string {
	switch {
	case access.ApplicationsErr != nil && access.ClustersErr != nil:
		return "applications or clusters"
	case access.ApplicationsErr != nil:
		return "applications"
	default:
		return "clusters"
	}
}
