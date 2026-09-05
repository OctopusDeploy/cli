package kubernetes

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// DefaultPreflightImage matches what the Octopus troubleshooting docs use for
// the same check by hand.
const DefaultPreflightImage = "busybox:1.37"

const (
	preflightPodPrefix = "octopus-preflight-"
	preflightTimeout   = 2 * time.Minute
	dialTimeout        = 5
)

type CheckResult int

const (
	CheckPassed CheckResult = iota
	CheckFailed
	// CheckSkipped means the check could not be run, which is not the same as
	// the endpoint being unreachable.
	CheckSkipped
)

func (r CheckResult) String() string {
	switch r {
	case CheckPassed:
		return "passed"
	case CheckFailed:
		return "failed"
	default:
		return "skipped"
	}
}

type Check struct {
	Name        string
	Result      CheckResult
	Detail      string
	Remediation string
}

type Target struct {
	Name        string
	Address     string
	Remediation string
}

// RESTAPITarget and GRPCTarget carry the remediation prose every component
// shares; purpose says what this component does over the endpoint.

func RESTAPITarget(host, purpose string) Target {
	return Target{
		Name:        "Octopus REST API",
		Address:     host,
		Remediation: purpose + " Confirm this address is reachable from inside the cluster.",
	}
}

func GRPCTarget(address, purpose string) Target {
	return Target{
		Name:    "Octopus gRPC endpoint",
		Address: address,
		Remediation: purpose + " A load balancer, proxy, or firewall that forwards only HTTPS is the usual cause; " +
			"make sure the gRPC port is forwarded too.",
	}
}

type PreflightRequest struct {
	Namespace string
	Image     string
	Targets   []Target
	// Warnings is where a failure to clean up the check pod is reported; the
	// checks themselves come back as values. Nil discards it.
	Warnings io.Writer
}

// StaticChecks need no cluster access, and catch the most common local-cluster
// mistake, so they run even when the pod-based checks are skipped.
func StaticChecks(targets []Target) []Check {
	checks := make([]Check, 0, len(targets))

	for _, t := range targets {
		host, _, err := splitTarget(t.Address)
		if err != nil {
			checks = append(checks, Check{
				Name:        t.Name,
				Result:      CheckFailed,
				Detail:      fmt.Sprintf("%q is not a valid address: %v", t.Address, err),
				Remediation: t.Remediation,
			})
			continue
		}

		if isLoopback(host) {
			checks = append(checks, Check{
				Name:   t.Name,
				Result: CheckFailed,
				Detail: fmt.Sprintf("%s resolves to this machine, not to a cluster-visible address", host),
				Remediation: "A pod cannot reach the loopback address of the machine running the CLI. " +
					"Use an address the cluster can resolve - local clusters usually provide a special hostname " +
					"such as host.docker.internal or host.minikube.internal.",
			})
		}
	}

	return checks
}

// RunPreflight covers only the pod-based targets; combine the result with
// StaticChecks.
func (c *Cluster) RunPreflight(ctx context.Context, req PreflightRequest) ([]Check, error) {
	if len(req.Targets) == 0 {
		return nil, nil
	}

	image := req.Image
	if image == "" {
		image = DefaultPreflightImage
	}

	pod, err := c.startPreflightPod(ctx, req, image)
	if err != nil {
		return nil, err
	}
	// Also on cancellation: a stray check pod left behind is our mess.
	defer c.deletePreflightPod(req.Warnings, pod.Namespace, pod.Name)

	if err := c.waitForPreflightPod(ctx, pod.Namespace, pod.Name); err != nil {
		return skippedChecks(req.Targets, err), nil
	}

	logs, err := c.preflightLogs(ctx, pod.Namespace, pod.Name)
	if err != nil {
		return skippedChecks(req.Targets, err), nil
	}

	return parsePreflightLogs(req.Targets, logs), nil
}

func (c *Cluster) startPreflightPod(ctx context.Context, req PreflightRequest, image string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: preflightPodPrefix,
			Namespace:    req.Namespace,
			Labels:       map[string]string{"app.kubernetes.io/managed-by": "octopus-cli"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "preflight",
				Image:   image,
				Command: []string{"sh", "-c", preflightScript(req.Targets)},
			}},
		},
	}

	created, err := c.Clientset.CoreV1().Pods(req.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not start the connectivity check pod in namespace %s: %w", req.Namespace, err)
	}
	return created, nil
}

// preflightScript emits one "<index> REACHABLE|UNREACHABLE" line per target, so
// every endpoint is covered by a single scheduling round trip.
func preflightScript(targets []Target) string {
	var b strings.Builder
	for i, t := range targets {
		host, port, err := splitTarget(t.Address)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "nc -z -w %d %s %s >/dev/null 2>&1 && echo '%d REACHABLE' || echo '%d UNREACHABLE'\n",
			dialTimeout, shellQuote(host), shellQuote(port), i, i)
	}
	return b.String()
}

func (c *Cluster) waitForPreflightPod(ctx context.Context, namespace, name string) error {
	ctx, cancel := context.WithTimeout(ctx, preflightTimeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return true, nil
		case corev1.PodPending:
			// A pod that cannot pull its image will never run, so do not wait
			// out the timeout for it.
			if reason := imagePullFailure(pod); reason != "" {
				return false, fmt.Errorf("the connectivity check pod could not start: %s. "+
					"Use --%s to choose an image this cluster can pull", reason, FlagPreflightImage)
			}
			return false, nil
		default:
			return false, nil
		}
	})
}

func imagePullFailure(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		w := cs.State.Waiting
		if w == nil {
			continue
		}
		if w.Reason == "ErrImagePull" || w.Reason == "ImagePullBackOff" || w.Reason == "InvalidImageName" {
			return strings.TrimSpace(w.Reason + ": " + w.Message)
		}
	}
	return ""
}

func (c *Cluster) preflightLogs(ctx context.Context, namespace, name string) (string, error) {
	stream, err := c.Clientset.CoreV1().Pods(namespace).
		GetLogs(name, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("could not read the connectivity check results: %w", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("could not read the connectivity check results: %w", err)
	}
	return string(body), nil
}

func (c *Cluster) deletePreflightPod(warnings io.Writer, namespace, name string) {
	// A fresh context: the caller's may already be cancelled, and the pod still
	// has to go.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grace := int64(0)
	err := c.Clientset.CoreV1().Pods(namespace).
		Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &grace})
	if err != nil && !apierrors.IsNotFound(err) && warnings != nil {
		fmt.Fprintf(warnings, "warning: could not delete the connectivity check pod %s/%s: %v\n", namespace, name, err)
	}
}

func parsePreflightLogs(targets []Target, logs string) []Check {
	reachable := map[int]bool{}
	for _, line := range strings.Split(logs, "\n") {
		var index int
		var status string
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d %s", &index, &status); err == nil {
			reachable[index] = status == "REACHABLE"
		}
	}

	checks := make([]Check, 0, len(targets))
	for i, t := range targets {
		ok, reported := reachable[i]
		switch {
		case !reported:
			checks = append(checks, Check{
				Name:   t.Name,
				Result: CheckSkipped,
				Detail: "the check pod did not report a result for this endpoint",
			})
		case ok:
			checks = append(checks, Check{Name: t.Name, Result: CheckPassed, Detail: t.Address})
		default:
			checks = append(checks, Check{
				Name:        t.Name,
				Result:      CheckFailed,
				Detail:      fmt.Sprintf("%s is not reachable from inside the cluster", t.Address),
				Remediation: t.Remediation,
			})
		}
	}
	return checks
}

func skippedChecks(targets []Target, err error) []Check {
	checks := make([]Check, 0, len(targets))
	for _, t := range targets {
		checks = append(checks, Check{Name: t.Name, Result: CheckSkipped, Detail: err.Error()})
	}
	return checks
}

// splitTarget accepts either a URL or a host:port.
func splitTarget(address string) (host string, port string, err error) {
	if strings.Contains(address, "://") {
		u, parseErr := url.Parse(address)
		if parseErr != nil {
			return "", "", parseErr
		}
		if u.Hostname() == "" {
			return "", "", fmt.Errorf("no host")
		}
		if p := u.Port(); p != "" {
			return u.Hostname(), p, nil
		}
		return u.Hostname(), defaultPortForScheme(u.Scheme), nil
	}

	host, port, err = net.SplitHostPort(address)
	if err != nil {
		return "", "", err
	}
	return host, port, nil
}

func defaultPortForScheme(scheme string) string {
	switch strings.ToLower(scheme) {
	case "http", "grpc":
		return "80"
	default:
		return "443"
	}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified()
	}
	return false
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
