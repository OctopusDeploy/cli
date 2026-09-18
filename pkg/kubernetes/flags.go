package kubernetes

import "time"

// Flag names live beside the cluster code rather than the commands, because
// error messages raised here tell the user which flag fixes the problem.
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
