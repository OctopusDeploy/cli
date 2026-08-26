package rotatetoken

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/argocd"
	"github.com/OctopusDeploy/cli/pkg/kubernetes/helm"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagRelease  = "release"
	FlagRestart  = "restart"
	gatewayChart = "octopus-argocd-gateway-chart"
	// gatewaySelector matches the gateway deployment the chart installs.
	gatewaySelector = "app.kubernetes.io/name=octopus-argocd-gateway"

	// The chart reads project tokens from Secret keys of this shape, with the
	// OCTOPUS_ARGOCD_ prefix added by envFrom.
	projectTokenEnvPrefix = "PROJECT_AUTH_TOKEN_"
	accountTokenEnvName   = "OCTOPUS_ARGOCD_AUTH_TOKEN"
)

type RotateFlags struct {
	Release *flag.Flag[string]
	Restart *flag.Flag[bool]

	*octoK8s.CommonFlags
}

func NewRotateFlags() *RotateFlags {
	return &RotateFlags{
		Release:     flag.New[string](FlagRelease, false),
		Restart:     flag.New[bool](FlagRestart, false),
		CommonFlags: octoK8s.NewCommonFlags(),
	}
}

type RotateOptions struct {
	*RotateFlags
	*cmd.Dependencies

	Cluster *octoK8s.Cluster
	Runner  *helm.Runner

	release    helm.Release
	deployment string
	instance   argocd.Instance
}

func NewCmdRotateToken(f factory.Factory) *cobra.Command {
	rotateFlags := NewRotateFlags()

	command := &cobra.Command{
		Use:   "rotate-token",
		Short: "Replace the Argo CD tokens an installed gateway uses",
		Long: heredoc.Doc(`
			Replace the Argo CD tokens an installed gateway uses.

			Shows every token the gateway holds and when it expires, links to where a
			replacement is generated, then checks the new one works against Argo CD before
			saving it.
		`),
		Example: heredoc.Docf("$ %s kubernetes gateway rotate-token", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := &RotateOptions{
				RotateFlags:  rotateFlags,
				Dependencies: cmd.NewDependencies(f, c),
			}
			return rotateRun(c.Context(), opts)
		},
	}

	flags := command.Flags()
	flags.SortFlags = false
	flags.StringVar(&rotateFlags.Release.Value, FlagRelease, "", "The gateway's Helm release name. Only needed when a cluster has more than one.")
	flags.BoolVar(&rotateFlags.Restart.Value, FlagRestart, true, "Restart the gateway so it picks up the new token.")
	octoK8s.RegisterCommonFlags(command, rotateFlags.CommonFlags)

	return command
}

func rotateRun(ctx context.Context, opts *RotateOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.NoPrompt {
		return errors.New("rotate-token replaces tokens that have to be generated in Argo CD by hand, so it cannot run with prompting disabled")
	}

	if err := opts.connect(ctx); err != nil {
		return err
	}
	if err := opts.selectRelease(); err != nil {
		return err
	}
	if err := opts.describeGateway(ctx); err != nil {
		return err
	}

	holdings, err := opts.currentTokens(ctx)
	if err != nil {
		return err
	}
	if len(holdings) == 0 {
		return fmt.Errorf("the %s gateway does not hold any Argo CD tokens to replace", opts.release.Name)
	}

	chosen, err := opts.selectTokens(holdings)
	if err != nil || len(chosen) == 0 {
		return err
	}

	for _, holding := range chosen {
		if err := opts.rotate(ctx, holding); err != nil {
			return err
		}
	}

	return opts.restart(ctx)
}

func (opts *RotateOptions) connect(ctx context.Context) error {
	kubeConfig, err := octoK8s.LoadKubeConfig(opts.KubeConfig.Value)
	if err != nil {
		return err
	}

	if opts.KubeContext.Value == "" {
		current, ok := kubeConfig.CurrentContext()
		if !ok {
			return fmt.Errorf("your kubeconfig has no current context, so --%s must be specified", octoK8s.FlagKubeContext)
		}
		opts.KubeContext.Value = current.Name
	}

	cluster, err := octoK8s.Connect(kubeConfig, opts.KubeContext.Value)
	if err != nil {
		return err
	}
	opts.Cluster = cluster

	runner, err := helm.NewRunner(opts.KubeConfig.Value, opts.KubeContext.Value, opts.Out)
	if err != nil {
		return err
	}
	opts.Runner = runner
	return nil
}

func (opts *RotateOptions) selectRelease() error {
	releases, err := opts.Runner.FindByChart(gatewayChart)
	if err != nil {
		return err
	}

	switch {
	case len(releases) == 0:
		return fmt.Errorf("no Octopus Argo CD gateway is installed in the %s cluster", opts.KubeContext.Value)
	case opts.Release.Value != "":
		for _, r := range releases {
			if r.Name == opts.Release.Value {
				opts.release = r
				return nil
			}
		}
		return fmt.Errorf("no gateway release named %q is installed in this cluster", opts.Release.Value)
	case len(releases) == 1:
		opts.release = releases[0]
		return nil
	}

	selected, err := question.SelectMap(opts.Ask, "Which gateway?", releases,
		func(r helm.Release) string { return fmt.Sprintf("%s (namespace %s)", r.Name, r.Namespace) })
	if err != nil {
		return err
	}
	opts.release = selected
	return nil
}

// describeGateway reads the gateway's own configuration back out of the
// cluster, so a token can be replaced without knowing how it was installed.
func (opts *RotateOptions) describeGateway(ctx context.Context) error {
	deployment, found, err := opts.Cluster.FindDeployment(ctx, opts.release.Namespace, gatewaySelector)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("the %s release has no gateway deployment in namespace %s", opts.release.Name, opts.release.Namespace)
	}
	opts.deployment = deployment.Name

	values, err := opts.Runner.GetValues(opts.release.Name, opts.release.Namespace)
	if err != nil {
		return err
	}
	opts.instance = instanceFromValues(values)

	fmt.Fprintf(opts.Out, "Gateway %s in namespace %s, connected to %s\n",
		output.Cyan(opts.release.Name), output.Cyan(opts.release.Namespace),
		output.Cyan(orUnknown(opts.instance.ServerGRPCURL)))
	return nil
}

// tokenHolding is one token the gateway reads, and where it is stored.
type tokenHolding struct {
	// Project is empty for the single account token an in-cluster gateway uses.
	Project    string
	SecretName string
	SecretKey  string
	Claims     argocd.ProjectTokenClaims
	// Parsed is false when the stored value is not a token this can read, which
	// is not a reason to refuse to replace it.
	Parsed bool
}

func (h tokenHolding) Display() string {
	name := h.Project
	if name == "" {
		name = "account token"
	}
	if !h.Parsed {
		return name
	}

	switch {
	case h.Claims.Expired():
		return fmt.Sprintf("%s - expired %s", name, h.Claims.Expires.Format("2 Jan 2006"))
	case !h.Claims.Expires.IsZero():
		return fmt.Sprintf("%s - expires %s", name, h.Claims.Expires.Format("2 Jan 2006"))
	default:
		return fmt.Sprintf("%s - does not expire", name)
	}
}

// currentTokens reads the gateway's deployment to find which Secrets hold its
// tokens. Taking it from the running workload rather than the Helm values means
// this works however the gateway was installed.
func (opts *RotateOptions) currentTokens(ctx context.Context) ([]tokenHolding, error) {
	deployment, found, err := opts.Cluster.FindDeployment(ctx, opts.release.Namespace, gatewaySelector)
	if err != nil || !found {
		return nil, err
	}

	var holdings []tokenHolding
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			ref := env.ValueFrom
			if env.Name != accountTokenEnvName || ref == nil || ref.SecretKeyRef == nil {
				continue
			}
			holdings = append(holdings, opts.readHolding(ctx, "", ref.SecretKeyRef.Name, ref.SecretKeyRef.Key))
		}

		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef == nil {
				continue
			}
			holdings = append(holdings, opts.readProjectHoldings(ctx, envFrom.SecretRef.Name)...)
		}
	}
	return holdings, nil
}

func (opts *RotateOptions) readProjectHoldings(ctx context.Context, secretName string) []tokenHolding {
	secret, found, err := opts.Cluster.GetSecret(ctx, opts.release.Namespace, secretName)
	if err != nil || !found {
		return nil
	}

	var holdings []tokenHolding
	for key := range secret.Data {
		project, isProjectToken := strings.CutPrefix(key, projectTokenEnvPrefix)
		if !isProjectToken {
			continue
		}
		holding := opts.readHolding(ctx, project, secretName, key)
		holdings = append(holdings, holding)
	}
	return holdings
}

func (opts *RotateOptions) readHolding(ctx context.Context, project, secretName, secretKey string) tokenHolding {
	holding := tokenHolding{Project: project, SecretName: secretName, SecretKey: secretKey}

	value, found, err := opts.Cluster.SecretKey(ctx, opts.release.Namespace, secretName, secretKey)
	if err != nil || !found || value == "" {
		return holding
	}

	if claims, err := argocd.ParseProjectToken(value); err == nil {
		holding.Claims, holding.Parsed = claims, true
		if holding.Project == "" {
			holding.Project = claims.Project
		}
	}
	return holding
}

func (opts *RotateOptions) selectTokens(holdings []tokenHolding) ([]tokenHolding, error) {
	if len(holdings) == 1 {
		return holdings, nil
	}
	return question.MultiSelectMap(opts.Ask, "Which tokens would you like to replace?", holdings,
		func(h tokenHolding) string { return h.Display() }, true)
}

func (opts *RotateOptions) rotate(ctx context.Context, holding tokenHolding) error {
	opts.printWhereToGenerate(holding)

	for {
		token := ""
		if err := opts.Ask(&survey.Password{
			Message: fmt.Sprintf("New token for %s", holdingName(holding)),
			Help:    "Paste the token Argo CD gave you.",
		}, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		if err := opts.validate(ctx, holding, token); err != nil {
			fmt.Fprintf(opts.Out, "  %s %v\n", output.Red("✘"), err)
			continue
		}

		err := opts.Cluster.MergeSecretKeys(ctx, opts.release.Namespace, holding.SecretName,
			map[string]string{holding.SecretKey: token}, nil)
		if err != nil {
			return err
		}

		fmt.Fprintf(opts.Out, "  %s Saved\n", output.Green("✔"))
		return nil
	}
}

func (opts *RotateOptions) printWhereToGenerate(holding tokenHolding) {
	fmt.Fprintf(opts.Out, "\n%s\n", output.Bold(holdingName(holding)))

	role := holding.Claims.Role
	if role == "" {
		role = argocd.DefaultAccountName
	}

	switch {
	case holding.Project != "":
		if url := argocd.ProjectRolePageURL(opts.instance.WebUIURL, holding.Project, role); url != "" {
			fmt.Fprintf(opts.Out, "  Generate a token for role %s at\n    %s\n", output.Cyan(role), output.Blue(url))
		}
		fmt.Fprintf(opts.Out, "  Or: %s\n",
			output.Cyan(fmt.Sprintf("argocd proj role create-token %s %s", holding.Project, role)))
	default:
		if opts.instance.WebUIURL != "" {
			fmt.Fprintf(opts.Out, "  Generate a token under Settings > Accounts > %s at\n    %s\n",
				output.Cyan(role), output.Blue(opts.instance.WebUIURL+"/settings/accounts"))
		}
		fmt.Fprintf(opts.Out, "  Or: %s\n",
			output.Cyan(fmt.Sprintf("argocd account generate-token --account %s", role)))
	}
}

// validate checks the replacement before it is stored, so a bad paste is caught
// here rather than by a gateway that silently stops working.
func (opts *RotateOptions) validate(ctx context.Context, holding tokenHolding, token string) error {
	claims, err := argocd.ParseProjectToken(token)
	switch {
	case err != nil && holding.Project != "":
		return err
	case err != nil:
		// An account token has a different subject shape, and cannot be
		// checked any further without reaching Argo CD.
		return opts.verifyAgainstArgoCD(ctx, token)
	}

	if claims.Expired() {
		return fmt.Errorf("this token expired on %s", claims.Expires.Format("2 Jan 2006"))
	}
	if holding.Project != "" && claims.Project != holding.Project {
		return fmt.Errorf("this token is for project %q, but %q is being replaced", claims.Project, holding.Project)
	}

	return opts.verifyAgainstArgoCD(ctx, token)
}

// verifyAgainstArgoCD proves the token actually works. Argo CD answers an
// under-privileged request with an empty list rather than a refusal, so a token
// that parses can still see nothing.
func (opts *RotateOptions) verifyAgainstArgoCD(ctx context.Context, token string) error {
	if opts.instance.WebUIURL == "" {
		return nil
	}

	client := argocd.NewClientForURL(opts.instance.WebUIURL)
	client.UseToken(token)

	access := client.VerifyAccess(ctx)
	if !access.Readable() {
		fmt.Fprintf(opts.Out, "  %s Argo CD would not let this token read applications. Saving it anyway.\n",
			output.Yellow("!"))
		return nil
	}

	fmt.Fprintf(opts.Out, "  %s\n", output.Dimf("Checked against Argo CD: reads %d %s.",
		access.Applications, octoK8s.Pluralise("application", "applications", access.Applications)))
	return nil
}

func (opts *RotateOptions) restart(ctx context.Context) error {
	if !opts.Restart.Value {
		fmt.Fprintf(opts.Out, "\nRestart the gateway for the new tokens to take effect:\n  %s\n",
			output.Cyan(fmt.Sprintf("kubectl rollout restart deploy/%s -n %s", opts.deployment, opts.release.Namespace)))
		return nil
	}

	if err := opts.Cluster.RestartDeployment(ctx, opts.release.Namespace, opts.deployment); err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "\n%s Restarted %s so it picks up the new tokens.\n",
		output.Green("✔"), output.Cyan(opts.deployment))
	return nil
}

func holdingName(holding tokenHolding) string {
	if holding.Project == "" {
		return "the gateway's Argo CD account token"
	}
	return "project " + holding.Project
}

func instanceFromValues(values map[string]any) argocd.Instance {
	gateway, _ := values["gateway"].(map[string]any)
	argo, _ := gateway["argocd"].(map[string]any)
	registration, _ := values["registration"].(map[string]any)
	registrationArgo, _ := registration["argocd"].(map[string]any)

	instance := argocd.Instance{}
	instance.ServerGRPCURL, _ = argo["serverGrpcUrl"].(string)
	instance.WebUIURL, _ = registrationArgo["webUiUrl"].(string)
	return instance
}

func orUnknown(value string) string {
	if value == "" {
		return "an unknown Argo CD"
	}
	return value
}
