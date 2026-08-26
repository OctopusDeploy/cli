package kubernetes

import (
	"fmt"
	"time"

	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagKubeConfig     = "kubeconfig"
	FlagKubeContext    = "kube-context"
	FlagNamespace      = "namespace"
	FlagReleaseName    = "release-name"
	FlagChartVersion   = "chart-version"
	FlagDryRun         = "dry-run"
	FlagOutputValues   = "output-values"
	FlagTimeout        = "timeout"
	FlagAtomic         = "atomic"
	FlagWait           = "wait"
	FlagSkipPreflight  = "skip-preflight"
	FlagPreflightImage = "preflight-image"
)

const DefaultTimeout = 10 * time.Minute

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
		KubeConfig:     flag.New[string](FlagKubeConfig, false),
		KubeContext:    flag.New[string](FlagKubeContext, false),
		Namespace:      flag.New[string](FlagNamespace, false),
		ReleaseName:    flag.New[string](FlagReleaseName, false),
		ChartVersion:   flag.New[string](FlagChartVersion, false),
		DryRun:         flag.New[bool](FlagDryRun, false),
		OutputValues:   flag.New[string](FlagOutputValues, false),
		Timeout:        flag.New[string](FlagTimeout, false),
		Atomic:         flag.New[bool](FlagAtomic, false),
		Wait:           flag.New[bool](FlagWait, false),
		SkipPreflight:  flag.New[bool](FlagSkipPreflight, false),
		PreflightImage: flag.New[string](FlagPreflightImage, false),
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
		return DefaultTimeout, nil
	}
	d, err := time.ParseDuration(f.Timeout.Value)
	if err != nil {
		return 0, fmt.Errorf("--%s must be a duration such as 5m or 90s: %w", FlagTimeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("--%s must be greater than zero", FlagTimeout)
	}
	return d, nil
}

func RegisterCommonFlags(cmd *cobra.Command, f *CommonFlags) {
	flags := cmd.Flags()
	flags.StringVar(&f.KubeConfig.Value, FlagKubeConfig, "", "Path to the kubeconfig file. Defaults to $KUBECONFIG, then ~/.kube/config.")
	flags.StringVar(&f.KubeContext.Value, FlagKubeContext, "", "The kubeconfig context to install into. Defaults to the current context.")
	flags.StringVar(&f.Namespace.Value, FlagNamespace, "", "The namespace to install into. Derived from the name if not set.")
	flags.StringVar(&f.ReleaseName.Value, FlagReleaseName, "", "The Helm release name. Derived from the name if not set.")
	flags.StringVar(&f.ChartVersion.Value, FlagChartVersion, "", "The chart version to install. Defaults to the latest compatible version.")
	flags.BoolVar(&f.DryRun.Value, FlagDryRun, false, "Render the manifests that would be applied, without installing anything.")
	flags.StringVarP(&f.OutputValues.Value, FlagOutputValues, "o", "", "Write the resolved Helm values to this file.")
	flags.StringVar(&f.Timeout.Value, FlagTimeout, "", fmt.Sprintf("How long to wait for the release to become ready, e.g. 5m. Defaults to %s.", DefaultTimeout))
	flags.BoolVar(&f.Atomic.Value, FlagAtomic, true, "Roll the release back if it fails to become ready.")
	flags.BoolVar(&f.Wait.Value, FlagWait, true, "Wait for the release's resources to become ready.")
	flags.BoolVar(&f.SkipPreflight.Value, FlagSkipPreflight, false, "Skip the connectivity checks that run before installing.")
	flags.StringVar(&f.PreflightImage.Value, FlagPreflightImage, DefaultPreflightImage, "The image used by the connectivity check pod.")
}

func Pluralise(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}
