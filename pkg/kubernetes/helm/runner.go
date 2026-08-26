// Package helm wraps the parts of the Helm SDK the Kubernetes installer needs,
// so command code deals in charts and values rather than Helm's action plumbing.
package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	chartv2loader "helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/storage/driver"
)

const storageDriver = "secret" // Helm's default

type ChartRef struct {
	Ref     string
	Version string
}

type Release struct {
	Name      string
	Namespace string
	Chart     string
	Version   string
	Revision  int
	Status    string
	Manifest  string
	Notes     string
}

type InstallSpec struct {
	Chart           ChartRef
	ReleaseName     string
	Namespace       string
	Values          map[string]any
	CreateNamespace bool
	Atomic          bool
	Wait            bool
	Timeout         time.Duration
	DryRun          bool
}

type Runner struct {
	settings *cli.EnvSettings
	registry *registry.Client
}

// NewRunner takes an empty kubeContext to mean the kubeconfig's current
// context, and an empty kubeConfigPath to mean the default loading rules.
func NewRunner(kubeConfigPath, kubeContext string, out io.Writer) (*Runner, error) {
	settings := cli.New()
	settings.KubeContext = kubeContext
	if kubeConfigPath != "" {
		settings.KubeConfig = kubeConfigPath
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(out),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create a Helm registry client: %w", err)
	}

	return &Runner{settings: settings, registry: registryClient}, nil
}

// configFor scopes Helm to a namespace.
//
// The namespace has to be set on the settings, not just passed to Init: the
// REST client getter built from them is what resolves the namespace for any
// manifest that does not name one, so without this a chart's resources are
// created in whatever namespace the kubeconfig context happens to point at
// rather than the one being installed into.
func (r *Runner) configFor(namespace string) (*action.Configuration, error) {
	if namespace != "" {
		r.settings.SetNamespace(namespace)
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(r.settings.RESTClientGetter(), namespace, storageDriver); err != nil {
		return nil, fmt.Errorf("could not initialise Helm: %w", err)
	}
	cfg.RegistryClient = r.registry
	return cfg, nil
}

func (r *Runner) List() ([]Release, error) {
	cfg, err := r.configFor("")
	if err != nil {
		return nil, err
	}

	list := action.NewList(cfg)
	list.All = true
	list.AllNamespaces = true
	list.SetStateMask()

	results, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("could not list Helm releases: %w", err)
	}

	releases := make([]Release, 0, len(results))
	for _, result := range results {
		rel, err := toRelease(result)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// GetValues returns a release's user-supplied values, so the installer can
// pre-populate its prompts from a previous install.
func (r *Runner) GetValues(releaseName, namespace string) (map[string]any, error) {
	cfg, err := r.configFor(namespace)
	if err != nil {
		return nil, err
	}

	values, err := action.NewGetValues(cfg).Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("could not read the values of release %q in namespace %q: %w", releaseName, namespace, err)
	}
	return values, nil
}

// Install upgrades when a release of the same name already exists, matching
// `helm upgrade --install`.
func (r *Runner) Install(ctx context.Context, spec InstallSpec) (Release, error) {
	cfg, err := r.configFor(spec.Namespace)
	if err != nil {
		return Release{}, err
	}

	installed, err := r.releaseExists(cfg, spec.ReleaseName)
	if err != nil {
		return Release{}, err
	}
	if installed {
		return r.upgrade(ctx, cfg, spec)
	}
	return r.install(ctx, cfg, spec)
}

// Render backs --dry-run.
func (r *Runner) Render(ctx context.Context, spec InstallSpec) (string, error) {
	spec.DryRun = true
	rel, err := r.Install(ctx, spec)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

func (r *Runner) install(ctx context.Context, cfg *action.Configuration, spec InstallSpec) (Release, error) {
	client := action.NewInstall(cfg)
	client.ReleaseName = spec.ReleaseName
	client.Namespace = spec.Namespace
	client.CreateNamespace = spec.CreateNamespace
	client.Version = spec.Chart.Version
	client.Timeout = spec.Timeout
	client.RollbackOnFailure = spec.Atomic
	client.WaitStrategy = waitStrategy(spec)
	client.SetRegistryClient(r.registry)
	if spec.DryRun {
		// Client-side only: a server-side dry run needs permissions the user
		// may not have, and fails for a namespace that does not exist yet.
		client.DryRunStrategy = action.DryRunClient
		client.CreateNamespace = false
	}

	chrt, err := r.loadChart(&client.ChartPathOptions, spec.Chart)
	if err != nil {
		return Release{}, err
	}

	result, err := client.RunWithContext(ctx, chrt, spec.Values)
	if err != nil {
		return Release{}, fmt.Errorf("helm install of %q failed: %w", spec.ReleaseName, err)
	}
	return toRelease(result)
}

func (r *Runner) upgrade(ctx context.Context, cfg *action.Configuration, spec InstallSpec) (Release, error) {
	client := action.NewUpgrade(cfg)
	client.Namespace = spec.Namespace
	client.Version = spec.Chart.Version
	client.Timeout = spec.Timeout
	client.RollbackOnFailure = spec.Atomic
	client.WaitStrategy = waitStrategy(spec)
	client.SetRegistryClient(r.registry)
	if spec.DryRun {
		client.DryRunStrategy = action.DryRunClient
	}

	chrt, err := r.loadChart(&client.ChartPathOptions, spec.Chart)
	if err != nil {
		return Release{}, err
	}

	result, err := client.RunWithContext(ctx, spec.ReleaseName, chrt, spec.Values)
	if err != nil {
		return Release{}, fmt.Errorf("helm upgrade of %q failed: %w", spec.ReleaseName, err)
	}
	return toRelease(result)
}

func (r *Runner) loadChart(pathOptions *action.ChartPathOptions, ref ChartRef) (chart.Charter, error) {
	pathOptions.Version = ref.Version

	path, err := pathOptions.LocateChart(ref.Ref, r.settings)
	if err != nil {
		return nil, fmt.Errorf("could not pull chart %s: %w", ref.Ref, err)
	}

	chrt, err := chartv2loader.Load(path)
	if err != nil {
		return nil, fmt.Errorf("could not load chart %s: %w", ref.Ref, err)
	}
	return chrt, nil
}

func (r *Runner) releaseExists(cfg *action.Configuration, releaseName string) (bool, error) {
	history := action.NewHistory(cfg)
	history.Max = 1

	switch _, err := history.Run(releaseName); {
	case err == nil:
		return true, nil
	case errors.Is(err, driver.ErrReleaseNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("could not check for an existing release named %q: %w", releaseName, err)
	}
}

func waitStrategy(spec InstallSpec) kube.WaitStrategy {
	// Atomic is meaningless without waiting - Helm has to know the release
	// failed before it can roll it back.
	if spec.Wait || spec.Atomic {
		return kube.StatusWatcherStrategy
	}
	return kube.HookOnlyStrategy
}

func toRelease(result release.Releaser) (Release, error) {
	accessor, err := release.NewAccessor(result)
	if err != nil {
		return Release{}, fmt.Errorf("could not read the Helm release: %w", err)
	}

	rel := Release{
		Name:      accessor.Name(),
		Namespace: accessor.Namespace(),
		Revision:  accessor.Version(),
		Status:    accessor.Status(),
		Manifest:  accessor.Manifest(),
		Notes:     accessor.Notes(),
	}

	if chartAccessor, err := chart.NewDefaultAccessor(accessor.Chart()); err == nil {
		if metadata := chartAccessor.MetadataAsMap(); metadata != nil {
			rel.Chart, _ = metadata["Name"].(string)
			rel.Version, _ = metadata["Version"].(string)
		}
	}
	return rel, nil
}

// FindByChart returns the installed releases of one chart, across all namespaces.
func (r *Runner) FindByChart(chartName string) ([]Release, error) {
	all, err := r.List()
	if err != nil {
		return nil, err
	}

	matches := make([]Release, 0, len(all))
	for _, release := range all {
		if release.Chart == chartName {
			matches = append(matches, release)
		}
	}
	return matches, nil
}
