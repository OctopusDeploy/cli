package kubernetes

import (
	"fmt"
	"path/filepath"
	"sort"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Context struct {
	Name      string
	Cluster   string
	Server    string
	Namespace string
	IsCurrent bool
	// EKS is set when the context authenticates through the AWS CLI, which is
	// how kubeconfig entries for EKS clusters are written.
	EKS *EKSContext
}

type EKSContext struct {
	ClusterName string
	Region      string
	Profile     string
}

func (c Context) Display() string {
	s := c.Name
	if c.Server != "" {
		s = fmt.Sprintf("%s (%s)", s, c.Server)
	}
	if c.IsCurrent {
		s += " [current]"
	}
	return s
}

type KubeConfig struct {
	path   string
	loader clientcmd.ClientConfigLoader
	raw    clientcmdapi.Config
}

// LoadKubeConfig honours $KUBECONFIG and the default path when explicitPath is
// empty.
func LoadKubeConfig(explicitPath string) (*KubeConfig, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if explicitPath != "" {
		rules.ExplicitPath = explicitPath
	}

	raw, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("could not load kubeconfig: %w", err)
	}

	return &KubeConfig{path: explicitPath, loader: rules, raw: *raw}, nil
}

func (k *KubeConfig) Contexts() []Context {
	contexts := make([]Context, 0, len(k.raw.Contexts))
	for name, ctx := range k.raw.Contexts {
		c := Context{
			Name:      name,
			Cluster:   ctx.Cluster,
			Namespace: ctx.Namespace,
			IsCurrent: name == k.raw.CurrentContext,
		}
		if cluster, ok := k.raw.Clusters[ctx.Cluster]; ok {
			c.Server = cluster.Server
		}
		if authInfo, ok := k.raw.AuthInfos[ctx.AuthInfo]; ok {
			c.EKS = eksContextFrom(authInfo)
		}
		contexts = append(contexts, c)
	}

	sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
	return contexts
}

func (k *KubeConfig) CurrentContext() (Context, bool) {
	if k.raw.CurrentContext == "" {
		return Context{}, false
	}
	for _, c := range k.Contexts() {
		if c.Name == k.raw.CurrentContext {
			return c, true
		}
	}
	return Context{}, false
}

func (k *KubeConfig) FindContext(name string) (Context, error) {
	for _, c := range k.Contexts() {
		if c.Name == name {
			return c, nil
		}
	}
	return Context{}, fmt.Errorf("no context named %q exists in the kubeconfig", name)
}

func (k *KubeConfig) Path() string {
	return k.path
}

// RestConfig builds a client-go configuration. An empty contextName uses the
// kubeconfig's current context.
func (k *KubeConfig) RestConfig(contextName string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(k.loader, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not build a Kubernetes client for context %q: %w", k.contextNameOrCurrent(contextName), err)
	}
	return cfg, nil
}

func (k *KubeConfig) contextNameOrCurrent(contextName string) string {
	if contextName != "" {
		return contextName
	}
	return k.raw.CurrentContext
}

// eksContextFrom reads the cluster name and region out of the AWS CLI call the
// kubeconfig makes to get a token. The AWS CLI is already required for the
// context to authenticate at all.
func eksContextFrom(authInfo *clientcmdapi.AuthInfo) *EKSContext {
	if authInfo == nil || authInfo.Exec == nil {
		return nil
	}

	command := filepath.Base(authInfo.Exec.Command)
	if command != "aws" && command != "aws.exe" {
		return nil
	}

	eks := &EKSContext{}
	args := authInfo.Exec.Args
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			break
		}
		switch args[i] {
		case "--cluster-name":
			eks.ClusterName = args[i+1]
		case "--region":
			eks.Region = args[i+1]
		case "--profile":
			eks.Profile = args[i+1]
		}
	}

	for _, env := range authInfo.Exec.Env {
		if env.Name == "AWS_PROFILE" && eks.Profile == "" {
			eks.Profile = env.Value
		}
	}

	if eks.ClusterName == "" {
		return nil
	}
	return eks
}
