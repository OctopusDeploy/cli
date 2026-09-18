package install

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

// Test-only exports: compiled into the test binary, never into the package.

func (opts *InstallOptions) RegistrationSecretForTest() (string, error) {
	return opts.registrationSecretContents()
}

func (opts *InstallOptions) RegisterForTest() error { return opts.register() }

func (opts *InstallOptions) DeregisterForTest(cause error) error { return opts.deregister(cause) }

func (opts *InstallOptions) ConfirmRetry(kubeConfig *octoK8s.KubeConfig, cause error) (bool, error) {
	return opts.connector().ConfirmRetry(kubeConfig, cause)
}

func PromptForProjectTokenForTest(opts *InstallOptions, project string) error {
	return promptForProjectToken(opts, project)
}

// RenderReviewForTest prints the review screen without asking anything.
func RenderReviewForTest(opts *InstallOptions) {
	_ = opts.resolveNames()
	shared.PrintReview(opts.Out, reviewGroups(opts))
}
