package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services/api"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

// McpPath is where the Octopus Server serves its remote MCP endpoint, relative to the server root
const McpPath = "/mcp"

const agentApiKeyPurpose = "Octopus CLI (MCP)"

// apiKeyHeader is the only header the MCP endpoint accepts AI agent API keys in; it deliberately rejects them as Bearer tokens
const apiKeyHeader = "X-Octopus-ApiKey"

// the server's 403 message when a valid API key that isn't an AI agent key is presented to the MCP endpoint
const agentApiKeyRequiredMessage = "requires an AI agent API key"

// Connect opens an MCP session against the Octopus Server's remote MCP endpoint.
//
// The MCP endpoint only accepts AI agent API keys, not the user API keys or access tokens the rest of the CLI uses.
// Unless one is supplied via OCTOPUS_MCP_API_KEY, we use the CLI's regular credentials to create an AI agent API key
// for the current user and save it to the CLI config for subsequent calls. If the saved key is later rejected
// (expired, revoked, or for a different server) we replace it once and retry.
func Connect(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer) (*mcp.ClientSession, error) {
	apiKey := viper.GetString(constants.ConfigMcpApiKey)
	_, apiKeyFromEnv := os.LookupEnv(constants.EnvOctopusMcpApiKey)

	savedKey := apiKey != ""

	if !savedKey {
		var err error
		if apiKey, err = createAgentApiKey(f, requester, log); err != nil {
			return nil, err
		}
	}

	session, rejected, err := connect(ctx, f, requester, apiKey)
	if err == nil {
		return session, nil
	}
	// only replace a key we saved earlier; a key from the environment is the user's to fix, and a new key won't fare better
	if !rejected || apiKeyFromEnv || !savedKey {
		return nil, err
	}

	fmt.Fprintln(log, "The saved MCP API key was rejected by the server; creating a new one.")
	if apiKey, err = createAgentApiKey(f, requester, log); err != nil {
		return nil, err
	}
	session, _, err = connect(ctx, f, requester, apiKey)
	return session, err
}

// connect attempts a single MCP connection. rejected is true if the server refused the API key itself.
func connect(ctx context.Context, f factory.Factory, requester apiclient.Requester, apiKey string) (session *mcp.ClientSession, rejected bool, err error) {
	baseHttpClient, err := f.GetHttpClient()
	if err != nil {
		return nil, false, err
	}
	baseTransport := http.DefaultTransport
	if baseHttpClient != nil && baseHttpClient.Transport != nil {
		baseTransport = baseHttpClient.Transport
	}

	auth := &apiKeyRoundTripper{
		next:          baseTransport,
		apiKey:        apiKey,
		requesterName: requester.GetRequester(),
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint:   strings.TrimSuffix(f.GetCurrentHost(), "/") + McpPath,
		HTTPClient: &http.Client{Transport: auth},
	}

	client := mcp.NewClient(&mcp.Implementation{Name: constants.ExecutableName, Title: "Octopus CLI"}, nil)
	session, err = client.Connect(ctx, transport, nil)
	if err != nil {
		if auth.rejection != "" {
			return nil, true, errors.New(auth.rejection)
		}
		return nil, false, err
	}
	return session, false, nil
}

func createAgentApiKey(f factory.Factory, requester apiclient.Requester, log io.Writer) (string, error) {
	client, err := f.GetSystemClient(requester)
	if err != nil {
		return "", err
	}

	me, err := client.Users.GetMe()
	if err != nil {
		return "", fmt.Errorf("cannot look up the current user to create an MCP API key: %w", err)
	}

	request := users.NewAPIKey(agentApiKeyPurpose, me.GetID())
	request.ActorType = users.APIKeyActorTypeAiAgent
	created, err := client.APIKeys.Create(request)
	if err != nil {
		return "", fmt.Errorf("cannot create an AI agent API key for MCP access: %w", err)
	}

	configProvider, err := f.GetConfigProvider()
	if err == nil {
		err = configProvider.Set(constants.ConfigMcpApiKey, created.APIKey)
	}
	if err != nil {
		fmt.Fprintf(log, "Created AI agent API key %s for MCP access, but could not save it to the CLI config (%v). A new key will be created next time.\n", created.GetID(), err)
	} else {
		fmt.Fprintf(log, "Created AI agent API key %s for MCP access and saved it to the CLI config.\n", created.GetID())
	}
	// the in-memory viper won't see the file write; set it so later lookups in this process agree
	viper.Set(constants.ConfigMcpApiKey, created.APIKey)

	return created.APIKey, nil
}

// apiKeyRoundTripper authenticates MCP requests and records whether the server rejected the API key
type apiKeyRoundTripper struct {
	next          http.RoundTripper
	apiKey        string
	requesterName string

	// the server's error message if it rejected the API key; empty otherwise
	rejection string
}

func (t *apiKeyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set(apiKeyHeader, t.apiKey)
	if t.requesterName != "" {
		req.Header.Set("User-Agent", api.GetUserAgentString(t.requesterName))
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil || (resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden) {
		return resp, err
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		return resp, nil
	}

	// a 403 can also mean remote MCP is switched off, which a new key won't fix
	if resp.StatusCode == http.StatusUnauthorized || strings.Contains(string(body), agentApiKeyRequiredMessage) {
		t.rejection = fmt.Sprintf("the Octopus Server rejected the MCP API key: %s", errorMessage(body))
	}
	return resp, nil
}

// errorMessage extracts the ErrorMessage from an Octopus API error response, falling back to the raw body
func errorMessage(body []byte) string {
	var apiError struct {
		ErrorMessage string `json:"ErrorMessage"`
	}
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.ErrorMessage != "" {
		return apiError.ErrorMessage
	}
	return strings.TrimSpace(string(body))
}
