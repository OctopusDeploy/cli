package argocd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

// awsTimeout keeps a stale SSO session showing up as a prompt for the endpoint
// rather than a hang.
const awsTimeout = 30 * time.Second

const argoCapabilityType = "ARGOCD"

// UnscopedProject names the token the gateway falls back on for Argo CD calls
// that are not project-scoped.
const UnscopedProject = "octo-gateway-unscoped"

// ProjectToken is a project role token. AWS caps account token lifetimes at 12
// hours, so managed instances authenticate per project instead.
type ProjectToken struct {
	Project string `json:"project"`
	Token   string `json:"token"`
}

// DiscoverEKSManaged asks AWS because the EKS capability runs Argo CD in the
// AWS control plane, not on the cluster's nodes - there is nothing in the
// cluster to find. The AWS CLI is already required for the kubeconfig context
// to authenticate, so calling it adds no new prerequisite.
func DiscoverEKSManaged(ctx context.Context, eks *octoK8s.EKSContext) (Instance, bool, error) {
	if eks == nil {
		return Instance{}, false, nil
	}
	if _, err := exec.LookPath("aws"); err != nil {
		return Instance{}, false, nil
	}

	capabilities, err := listArgoCapabilities(ctx, eks)
	if err != nil {
		return Instance{}, false, err
	}

	for _, capability := range capabilities {
		instance, found, err := describeArgoCapability(ctx, eks, capability)
		if err != nil {
			return Instance{}, false, err
		}
		if found {
			return instance, true, nil
		}
	}
	return Instance{}, false, nil
}

// NewManagedInstance keeps TLS verification on and tunnels gRPC over HTTP/1.1:
// AWS serves managed Argo CD with a publicly trusted certificate, behind a load
// balancer that does not support HTTP/2.
func NewManagedInstance(endpoint string) Instance {
	return Instance{
		Kind:          KindEKSManaged,
		ServerGRPCURL: normaliseGRPCURL(endpoint),
		Plaintext:     false,
		SelfSignedTLS: false,
		GRPCWeb:       true,
	}
}

type capabilitySummary struct {
	CapabilityName string `json:"capabilityName"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Version        string `json:"version"`
}

func listArgoCapabilities(ctx context.Context, eks *octoK8s.EKSContext) ([]capabilitySummary, error) {
	out, err := runAWS(ctx, eks, "eks", "list-capabilities", "--cluster-name", eks.ClusterName)
	if err != nil {
		return nil, err
	}

	capabilities, err := parseCapabilityList(out)
	if err != nil {
		return nil, fmt.Errorf("could not read the EKS capabilities for cluster %s: %w", eks.ClusterName, err)
	}
	return capabilities, nil
}

// parseCapabilityList picks out the Argo CD entries. The listing carries the
// type and version, so only those need a follow-up describe call.
func parseCapabilityList(out []byte) ([]capabilitySummary, error) {
	var response struct {
		Capabilities []capabilitySummary `json:"capabilities"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, err
	}

	var argo []capabilitySummary
	for _, c := range response.Capabilities {
		if strings.EqualFold(c.Type, argoCapabilityType) {
			argo = append(argo, c)
		}
	}
	return argo, nil
}

func describeArgoCapability(ctx context.Context, eks *octoK8s.EKSContext, summary capabilitySummary) (Instance, bool, error) {
	out, err := runAWS(ctx, eks, "eks", "describe-capability",
		"--cluster-name", eks.ClusterName, "--capability-name", summary.CapabilityName)
	if err != nil {
		return Instance{}, false, err
	}

	instance, found, err := parseCapabilityDescription(out, summary)
	if err != nil {
		return Instance{}, false, fmt.Errorf("could not read the %s capability on cluster %s: %w",
			summary.CapabilityName, eks.ClusterName, err)
	}
	return instance, found, nil
}

// parseCapabilityDescription reads the address out of `aws eks
// describe-capability`.
func parseCapabilityDescription(out []byte, summary capabilitySummary) (Instance, bool, error) {
	var response struct {
		Capability struct {
			Status        string `json:"status"`
			Version       string `json:"version"`
			Configuration struct {
				ArgoCD struct {
					Namespace string `json:"namespace"`
					ServerURL string `json:"serverUrl"`
				} `json:"argoCd"`
			} `json:"configuration"`
		} `json:"capability"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return Instance{}, false, err
	}

	argo := response.Capability.Configuration.ArgoCD
	if strings.TrimSpace(argo.ServerURL) == "" {
		// No address yet, which is what a still-provisioning capability looks like.
		return Instance{}, false, nil
	}

	instance := NewManagedInstance(argo.ServerURL)
	instance.Name = summary.CapabilityName
	instance.Namespace = argo.Namespace
	instance.Version = firstNonEmpty(response.Capability.Version, summary.Version)
	instance.Status = firstNonEmpty(response.Capability.Status, summary.Status)
	instance.WebUIURL = webURLFor(argo.ServerURL)
	return instance, true, nil
}

func runAWS(ctx context.Context, eks *octoK8s.EKSContext, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, awsTimeout)
	defer cancel()

	full := append([]string{}, args...)
	if eks.Region != "" {
		full = append(full, "--region", eks.Region)
	}
	full = append(full, "--output", "json")

	cmd := exec.CommandContext(ctx, "aws", full...)
	cmd.Env = os.Environ()
	if eks.Profile != "" {
		cmd.Env = append(cmd.Env, "AWS_PROFILE="+eks.Profile)
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("aws %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// normaliseGRPCURL produces the grpc:// URL the chart expects. AWS reports the
// endpoint as a bare hostname or an https:// URL depending where it is read from.
func normaliseGRPCURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.HasPrefix(endpoint, "grpc://") {
		return endpoint
	}

	host := endpoint
	for _, prefix := range []string{"https://", "http://"} {
		host = strings.TrimPrefix(host, prefix)
	}
	return "grpc://" + strings.TrimSuffix(host, "/")
}

func webURLFor(endpoint string) string {
	host := strings.TrimPrefix(normaliseGRPCURL(endpoint), "grpc://")
	if host == "" {
		return ""
	}
	return "https://" + host
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ProjectTokenClaims are the parts of an Argo CD project role token Octopus
// needs to know.
type ProjectTokenClaims struct {
	Project string
	Role    string
	// Expires is zero for a token that does not expire.
	Expires time.Time
}

// Expired reports whether the token has already lapsed.
func (c ProjectTokenClaims) Expired() bool {
	return !c.Expires.IsZero() && c.Expires.Before(time.Now())
}

// ParseProjectToken reads the project and role out of a token without verifying
// it. Only Argo CD can verify one, and all that is needed here is the subject,
// which Argo CD sets to proj:<project>:<role> - so a person pasting a token
// does not also have to say which project it belongs to.
func ParseProjectToken(token string) (ProjectTokenClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return ProjectTokenClaims{}, fmt.Errorf("this does not look like an Argo CD token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ProjectTokenClaims{}, fmt.Errorf("this does not look like an Argo CD token")
	}

	var claims struct {
		Subject string `json:"sub"`
		Expires int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ProjectTokenClaims{}, fmt.Errorf("this does not look like an Argo CD token")
	}

	project, role, err := splitProjectSubject(claims.Subject)
	if err != nil {
		return ProjectTokenClaims{}, err
	}

	parsed := ProjectTokenClaims{Project: project, Role: role}
	if claims.Expires > 0 {
		parsed.Expires = time.Unix(claims.Expires, 0)
	}
	return parsed, nil
}

// splitProjectSubject rejects anything that is not a project role token. An
// account token has a subject of the form <account>:apiKey, and AWS caps those
// at 12 hours, so one here would stop working within the day.
func splitProjectSubject(subject string) (project, role string, err error) {
	parts := strings.Split(subject, ":")
	if len(parts) != 3 || parts[0] != "proj" || parts[1] == "" || parts[2] == "" {
		return "", "", fmt.Errorf(
			"this is not an Argo CD project role token (its subject is %q). Generate one with "+
				"`argocd proj role create-token <project> <role>`, or from Settings > Projects in the Argo CD UI", subject)
	}
	return parts[1], parts[2], nil
}
