package install

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/shared"
)

// Test-only exports: compiled into the test binary, never into the package.

func ReportFindingsForTest(opts *InstallOptions) {
	opts.reportFindings()
}

// RenderReviewForTest prints the review screen without asking anything.
func RenderReviewForTest(opts *InstallOptions) {
	_ = opts.resolveNames()
	shared.PrintReview(opts.Out, reviewGroups(opts))
}

func (opts *InstallOptions) NewTargetTagsForTest() []string {
	return opts.newTargetTags()
}

func (opts *InstallOptions) DeriveAccessModeForTest() {
	opts.deriveAccessMode()
}
