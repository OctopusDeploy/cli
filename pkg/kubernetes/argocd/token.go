package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

type Credentials struct {
	Username string
	Password string
}

const AdminUsername = "admin"

// Argo CD watches argocd-cm and reloads it in place, so noticing a change is
// normally immediate.
const accountPropagationTimeout = 60 * time.Second

// InitialAdminPassword treats a missing Secret as an ordinary outcome: Argo CD
// tells administrators to delete it once they have logged in.
func InitialAdminPassword(ctx context.Context, c *octoK8s.Cluster, namespace string) (string, bool, error) {
	secret, found, err := c.GetSecret(ctx, namespace, InitialAdminSecretName)
	if err != nil || !found {
		return "", false, err
	}
	password, ok := secret.Data["password"]
	if !ok || len(password) == 0 {
		return "", false, nil
	}
	return string(password), true, nil
}

type Client struct {
	baseURL string
	http    *http.Client
	token   string
}

// Dial port-forwards to the API server pod rather than requiring an ingress,
// because a stock Argo CD exposes its API in-cluster only. The returned
// function tears the port-forward down.
func Dial(ctx context.Context, c *octoK8s.Cluster, restConfig *rest.Config, instance Instance) (*Client, func(), error) {
	forwarder, localPort, err := startPortForward(ctx, c, restConfig, instance)
	if err != nil {
		return nil, nil, err
	}

	client := &Client{
		baseURL: fmt.Sprintf("https://127.0.0.1:%d", localPort),
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// A local port-forward to a certificate that is self-signed by
				// default: there is nothing here worth verifying.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
	if instance.Plaintext {
		client.baseURL = fmt.Sprintf("http://127.0.0.1:%d", localPort)
	}

	return client, forwarder, nil
}

func startPortForward(ctx context.Context, c *octoK8s.Cluster, restConfig *rest.Config, instance Instance) (func(), int, error) {
	// Match the pods of the deployment discovery already found, rather than
	// guessing at labels again.
	deployment, err := c.Clientset.AppsV1().Deployments(instance.Namespace).
		Get(ctx, instance.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("could not find the Argo CD API server: %w", err)
	}

	selector := labels.Set(deployment.Spec.Selector.MatchLabels).String()
	pods, err := c.Clientset.CoreV1().Pods(instance.Namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, 0, fmt.Errorf("could not find the Argo CD API server pod: %w", err)
	}

	podName := ""
	for _, p := range pods.Items {
		if p.Status.Phase == "Running" {
			podName = p.Name
			break
		}
	}
	if podName == "" {
		return nil, 0, fmt.Errorf("no running Argo CD API server pod was found in namespace %s", instance.Namespace)
	}

	// argocd-server listens on 8080 inside the pod whether or not it serves
	// TLS; the Service maps 80/443 onto it.
	const targetPort = 8080

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("could not prepare a port-forward to Argo CD: %w", err)
	}

	host := strings.TrimPrefix(strings.TrimPrefix(restConfig.Host, "https://"), "http://")
	forwardURL := &url.URL{
		Scheme: "https",
		Path:   fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", instance.Namespace, podName),
		Host:   host,
	}

	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, forwardURL)
	forwarder, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf(":%d", targetPort)}, // empty local port: let the OS pick
		stopCh, readyCh, io.Discard, io.Discard,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("could not prepare a port-forward to Argo CD: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- forwarder.ForwardPorts() }()

	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, 0, fmt.Errorf("could not port-forward to Argo CD: %w", err)
	case <-time.After(30 * time.Second):
		close(stopCh)
		return nil, 0, fmt.Errorf("timed out port-forwarding to Argo CD in namespace %s", instance.Namespace)
	case <-ctx.Done():
		close(stopCh)
		return nil, 0, ctx.Err()
	}

	ports, err := forwarder.GetPorts()
	if err != nil || len(ports) == 0 {
		close(stopCh)
		return nil, 0, fmt.Errorf("could not determine the local port-forward port: %w", err)
	}

	return func() { close(stopCh) }, int(ports[0].Local), nil
}

func (c *Client) Login(ctx context.Context, creds Credentials) error {
	var response struct {
		Token string `json:"token"`
	}

	body := map[string]string{"username": creds.Username, "password": creds.Password}
	if err := c.do(ctx, http.MethodPost, "/api/v1/session", body, &response); err != nil {
		return fmt.Errorf("could not sign in to Argo CD as %q: %w", creds.Username, err)
	}
	if response.Token == "" {
		return fmt.Errorf("Argo CD did not return a session token for %q", creds.Username)
	}

	c.token = response.Token
	return nil
}

// AccountExists can be false for a moment after the account is added: Argo CD
// reloads argocd-cm in the background.
func (c *Client) AccountExists(ctx context.Context, name string) (bool, error) {
	var response struct {
		Items []struct {
			Name         string   `json:"name"`
			Enabled      bool     `json:"enabled"`
			Capabilities []string `json:"capabilities"`
		} `json:"items"`
	}

	if err := c.do(ctx, http.MethodGet, "/api/v1/account", nil, &response); err != nil {
		return false, err
	}

	for _, a := range response.Items {
		if a.Name != name {
			continue
		}
		for _, capability := range a.Capabilities {
			if strings.EqualFold(capability, "apiKey") {
				return a.Enabled, nil
			}
		}
	}
	return false, nil
}

func (c *Client) WaitForAccount(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, accountPropagationTimeout)
	defer cancel()

	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		// Errors are expected while argocd-server reloads its configuration.
		exists, err := c.AccountExists(ctx, name)
		if err != nil {
			return false, nil
		}
		return exists, nil
	})

	if err != nil {
		return fmt.Errorf("Argo CD did not pick up the %q account within %s. "+
			"Restarting the argocd-server deployment usually resolves this", name, accountPropagationTimeout)
	}
	return nil
}

// GenerateToken mints a non-expiring API key, matching
// `argocd account generate-token`.
func (c *Client) GenerateToken(ctx context.Context, accountName string) (string, error) {
	var response struct {
		Token string `json:"token"`
	}

	path := fmt.Sprintf("/api/v1/account/%s/token", url.PathEscape(accountName))
	if err := c.do(ctx, http.MethodPost, path, map[string]any{}, &response); err != nil {
		return "", fmt.Errorf("could not generate an Argo CD token for %q: %w", accountName, err)
	}
	if response.Token == "" {
		return "", fmt.Errorf("Argo CD did not return a token for %q", accountName)
	}
	return response.Token, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the Argo CD API: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Argo CD returned %s: %s", resp.Status, argoErrorMessage(responseBody))
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(responseBody, out)
}

// argoErrorMessage falls back to the raw body when the envelope has no message.
func argoErrorMessage(body []byte) string {
	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Message != "" {
			return envelope.Message
		}
		if envelope.Error != "" {
			return envelope.Error
		}
	}
	return strings.TrimSpace(string(body))
}

func (c *Client) UseToken(token string) {
	c.token = token
}

type AccessCheck struct {
	Applications    int
	Clusters        int
	ApplicationsErr error
	ClustersErr     error
}

func (a AccessCheck) Readable() bool {
	return a.ApplicationsErr == nil && a.ClustersErr == nil
}

// VerifyAccess exists because Argo CD answers an under-privileged request with
// an empty list rather than an error, so a gateway can connect happily and then
// show nothing at all.
func (c *Client) VerifyAccess(ctx context.Context) AccessCheck {
	var check AccessCheck

	applications, err := c.listNames(ctx, "/api/v1/applications")
	check.Applications, check.ApplicationsErr = len(applications), err

	clusters, err := c.listNames(ctx, "/api/v1/clusters")
	check.Clusters, check.ClustersErr = len(clusters), err

	return check
}

func (c *Client) ListApplicationNames(ctx context.Context) ([]string, error) {
	return c.listNames(ctx, "/api/v1/applications")
}

func (c *Client) listNames(ctx context.Context, path string) ([]string, error) {
	var response struct {
		Items []struct {
			Name     string `json:"name"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		names = append(names, firstNonEmpty(item.Metadata.Name, item.Name))
	}
	return names, nil
}

// NewClientForURL talks to an Argo CD that is already reachable, which is the
// case for the AWS managed capability. An in-cluster Argo CD needs Dial.
func NewClientForURL(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
		},
	}
}
