package shared

import (
	"fmt"
	"time"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

// CommonFlags are embedded by each component's own flags struct.
type CommonFlags struct {
	KubeConfig     *flag.Flag[string]
	KubeContext    *flag.Flag[string]
	Namespace      *flag.Flag[string]
	ReleaseName    *flag.Flag[string]
	ChartVersion   *flag.Flag[string]
	DryRun         *flag.Flag[bool]
	OutputValues   *flag.Flag[string]
	Timeout        *flag.Flag[string]
	Atomic         *flag.Flag[bool]
	Wait           *flag.Flag[bool]
	SkipPreflight  *flag.Flag[bool]
	PreflightImage *flag.Flag[string]
}

func NewCommonFlags() *CommonFlags {
	return &CommonFlags{
		KubeConfig:     flag.New[string](octoK8s.FlagKubeConfig, false),
		KubeContext:    flag.New[string](octoK8s.FlagKubeContext, false),
		Namespace:      flag.New[string](octoK8s.FlagNamespace, false),
		ReleaseName:    flag.New[string](octoK8s.FlagReleaseName, false),
		ChartVersion:   flag.New[string](octoK8s.FlagChartVersion, false),
		DryRun:         flag.New[bool](octoK8s.FlagDryRun, false),
		OutputValues:   flag.New[string](octoK8s.FlagOutputValues, false),
		Timeout:        flag.New[string](octoK8s.FlagTimeout, false),
		Atomic:         flag.New[bool](octoK8s.FlagAtomic, false),
		Wait:           flag.New[bool](octoK8s.FlagWait, false),
		SkipPreflight:  flag.New[bool](octoK8s.FlagSkipPreflight, false),
		PreflightImage: flag.New[string](octoK8s.FlagPreflightImage, false),
	}
}

func (f *CommonFlags) Generatable() []flag.Generatable {
	return []flag.Generatable{
		f.KubeConfig, f.KubeContext, f.Namespace, f.ReleaseName, f.ChartVersion,
		f.Timeout, f.Atomic, f.Wait, f.SkipPreflight, f.PreflightImage,
	}
}

func (f *CommonFlags) ResolveTimeout() (time.Duration, error) {
	if f.Timeout.Value == "" {
		return octoK8s.DefaultTimeout, nil
	}
	d, err := time.ParseDuration(f.Timeout.Value)
	if err != nil {
		return 0, fmt.Errorf("--%s must be a duration such as 5m or 90s: %w", octoK8s.FlagTimeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("--%s must be greater than zero", octoK8s.FlagTimeout)
	}
	return d, nil
}

// CommonFlagDetails adapts the shared flag help to the component, so no
// command has to patch usage strings after registration.
type CommonFlagDetails struct {
	// NamespaceDefault and ReleaseDefault say what happens when the flag is
	// not given.
	NamespaceDefault string
	ReleaseDefault   string
	// Checks names what --skip-preflight skips.
	Checks string
	// NoCheckPod hides --preflight-image for a component whose checks start no pod.
	NoCheckPod bool
}

// DerivedFromNameDetails fits the components whose namespace and release
// follow the name they register with.
func DerivedFromNameDetails() CommonFlagDetails {
	return CommonFlagDetails{
		NamespaceDefault: "Derived from the name if not set.",
		ReleaseDefault:   "Derived from the name if not set.",
		Checks:           "connectivity",
	}
}

func RegisterCommonFlags(cmd *cobra.Command, f *CommonFlags, details CommonFlagDetails) {
	flags := cmd.Flags()
	flags.StringVar(&f.KubeConfig.Value, octoK8s.FlagKubeConfig, "", "Path to the kubeconfig file. Defaults to $KUBECONFIG, then ~/.kube/config.")
	flags.StringVar(&f.KubeContext.Value, octoK8s.FlagKubeContext, "", "The kubeconfig context to install into. Defaults to the current context.")
	flags.StringVar(&f.Namespace.Value, octoK8s.FlagNamespace, "", "The namespace to install into. "+details.NamespaceDefault)
	flags.StringVar(&f.ReleaseName.Value, octoK8s.FlagReleaseName, "", "The Helm release name. "+details.ReleaseDefault)
	flags.StringVar(&f.ChartVersion.Value, octoK8s.FlagChartVersion, "", "The chart version to install. Defaults to the latest compatible version.")
	flags.BoolVar(&f.DryRun.Value, octoK8s.FlagDryRun, false, "Render the manifests that would be applied, without installing anything.")
	flags.StringVarP(&f.OutputValues.Value, octoK8s.FlagOutputValues, "o", "", "Write the resolved Helm values to this file.")
	flags.StringVar(&f.Timeout.Value, octoK8s.FlagTimeout, "", fmt.Sprintf("How long to wait for the release to become ready, e.g. 5m. Defaults to %s.", octoK8s.DefaultTimeout))
	flags.BoolVar(&f.Atomic.Value, octoK8s.FlagAtomic, true, "Roll the release back if it fails to become ready.")
	flags.BoolVar(&f.Wait.Value, octoK8s.FlagWait, true, "Wait for the release's resources to become ready.")
	flags.BoolVar(&f.SkipPreflight.Value, octoK8s.FlagSkipPreflight, false, fmt.Sprintf("Skip the %s checks that run before installing.", details.Checks))
	flags.StringVar(&f.PreflightImage.Value, octoK8s.FlagPreflightImage, octoK8s.DefaultPreflightImage, "The image used by the connectivity check pod.")
	if details.NoCheckPod {
		_ = flags.MarkHidden(octoK8s.FlagPreflightImage)
	}
}
