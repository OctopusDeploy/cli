package install

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
)

// Test-only exports: compiled into the test binary, never into the package.

func (opts *InstallOptions) ReportSuccessForTest(release helm.Release) {
	opts.reportSuccess(release)
}

func (opts *InstallOptions) ConfirmPrerequisitesForTest() error {
	return opts.confirmPrerequisites()
}

func (opts *InstallOptions) ResolveNamesForTest() {
	opts.resolveNames()
}

func RenderReviewForTest(opts *InstallOptions) {
	opts.resolveNames()
	shared.PrintReview(opts.Out, reviewGroups(opts))
}
