package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
)

const (
	agentRequiredError = "The Octopus MCP endpoint requires an AI agent API key supplied in the X-Octopus-ApiKey header."
	invalidKeyError    = "Invalid API key."
	mcpDisabledError   = "Remote MCP is disabled. An administrator can enable it in Configuration > Settings > MCP Controls."
)

// fakeOctopus answers the requests the MCP client makes through a MockHttpServer: the REST calls used to create an
// AI agent API key, and the MCP endpoint itself, which it serves with a real MCP server behind the Octopus Server's
// API key checks.
//
// The MCP client issues several requests per call, some of them concurrently, so rather than scripting each one with
// ExpectRequest we answer whatever arrives and let tests assert on what was recorded.
type fakeOctopus struct {
	t   *testing.T
	api *testutil.MockHttpServer
	mcp http.Handler

	mu sync.Mutex
	// keys the MCP endpoint accepts
	agentKeys map[string]bool
	// valid keys that aren't AI agent keys, which the MCP endpoint refuses with a 403
	userKeys map[string]bool
	// if set, keys created through the API aren't accepted by the MCP endpoint
	rejectCreatedKeys bool
	// if set, the MCP endpoint refuses every request as it does when remote MCP is switched off
	mcpDisabled bool

	createdKeys   []*users.CreateAPIKey
	mcpKeysSeen   []string
	toolArguments map[string]any
}

func newFakeOctopus(t *testing.T) *fakeOctopus {
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-octopus"}, nil)
	fake := &fakeOctopus{
		t:         t,
		api:       testutil.NewMockHttpServer(),
		agentKeys: map[string]bool{},
		userKeys:  map[string]bool{},
	}
	fake.mcp = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})

	fake.addTool(server, "get_current_user", nil)
	fake.addTool(server, "find_projects", []string{"SpaceId"}, "SpaceId", "PartialName")
	fake.addTool(server, "create_release", []string{"SpaceId", "SpaceIdOrName", "ProjectName"}, "SpaceId", "SpaceIdOrName", "ProjectName")
	fake.addTool(server, "find_events", nil, "SpaceId")

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case r := <-fake.api.Request:
				fake.api.Respond(fake.serve(r), nil)
			case <-done:
				return
			}
		}
	}()
	return fake
}

// addTool adds a tool with string parameters that records the arguments it's called with
func (f *fakeOctopus) addTool(server *mcp.Server, name string, required []string, properties ...string) {
	schemaProperties := map[string]any{}
	for _, property := range properties {
		schemaProperties[property] = map[string]any{"type": "string"}
	}
	schema := map[string]any{"type": "object", "properties": schemaProperties}
	if required != nil {
		schema["required"] = required
	}

	server.AddTool(&mcp.Tool{Name: name, InputSchema: schema}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := map[string]any{}
		if err := json.Unmarshal(req.Params.Arguments, &arguments); err != nil {
			return nil, err
		}
		f.mu.Lock()
		f.toolArguments = arguments
		f.mu.Unlock()
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
}

func (f *fakeOctopus) serve(r *http.Request) *http.Response {
	if r.URL.Path == McpPath {
		return f.serveMcp(r)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && (path == "/api" || path == "/api/spaces" || path == "/api/Spaces-1"):
		return jsonResponse(http.StatusOK, testutil.NewRootResource())
	case r.Method == http.MethodGet && path == "/api/users/me":
		user := users.NewUser("alice", "Alice")
		user.ID = "Users-1"
		return jsonResponse(http.StatusOK, user)
	case r.Method == http.MethodPost && path == "/api/users/Users-1/apikeys":
		request := &users.CreateAPIKey{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"ErrorMessage": err.Error()})
		}
		f.createdKeys = append(f.createdKeys, request)
		created := *request
		created.ID = fmt.Sprintf("apikeys-%d", len(f.createdKeys))
		created.APIKey = fmt.Sprintf("API-AGENT%d", len(f.createdKeys))
		if !f.rejectCreatedKeys {
			f.agentKeys[created.APIKey] = true
		}
		return jsonResponse(http.StatusCreated, created)
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		return jsonResponse(http.StatusNotFound, nil)
	}
}

func (f *fakeOctopus) serveMcp(r *http.Request) *http.Response {
	if refusal := f.checkMcpApiKey(r.Header.Get(apiKeyHeader)); refusal != nil {
		return refusal
	}

	// not under the lock; the tool handlers take it
	recorder := httptest.NewRecorder()
	f.mcp.ServeHTTP(recorder, r)
	return recorder.Result()
}

// checkMcpApiKey emulates the Octopus Server's MCP authentication, returning the response it refuses the key with, if any
func (f *fakeOctopus) checkMcpApiKey(key string) *http.Response {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mcpKeysSeen = append(f.mcpKeysSeen, key)

	switch {
	case f.mcpDisabled:
		return jsonResponse(http.StatusForbidden, map[string]string{"ErrorMessage": mcpDisabledError})
	case f.userKeys[key]:
		return jsonResponse(http.StatusForbidden, map[string]string{"ErrorMessage": agentRequiredError})
	case !f.agentKeys[key]:
		return jsonResponse(http.StatusUnauthorized, map[string]string{"ErrorMessage": invalidKeyError})
	}
	return nil
}

func (f *fakeOctopus) createdKeyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.createdKeys)
}

// mcpKeys returns the distinct API keys presented to the MCP endpoint, in the order they were first seen
func (f *fakeOctopus) mcpKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	for _, key := range f.mcpKeysSeen {
		if len(keys) == 0 || keys[len(keys)-1] != key {
			keys = append(keys, key)
		}
	}
	return keys
}

func jsonResponse(status int, body any) *http.Response {
	var encoded []byte
	if body != nil {
		encoded, _ = json.Marshal(body)
	}
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(encoded)),
		ContentLength: int64(len(encoded)),
	}
}

// resetViper isolates a test from the global viper the MCP client reads its saved key and space from
func resetViper(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func connectWith(t *testing.T, f *testutil.MockFactory) (*mcp.ClientSession, string, error) {
	var log bytes.Buffer
	session, err := Connect(context.Background(), f, &apiclient.FakeRequesterContext{}, &log)
	if session != nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	return session, log.String(), err
}

func TestConnect_CreatesAndSavesAgentKeyWhenNoneSaved(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	f := testutil.NewMockFactory(fake.api)

	session, log, err := connectWith(t, f)
	require.NoError(t, err)

	require.Equal(t, 1, fake.createdKeyCount())
	assert.Equal(t, users.APIKeyActorTypeAiAgent, fake.createdKeys[0].ActorType)
	assert.Equal(t, agentApiKeyPurpose, fake.createdKeys[0].Purpose)
	assert.Equal(t, "Users-1", fake.createdKeys[0].UserID)

	assert.Equal(t, "API-AGENT1", f.ConfigProvider.Get(constants.ConfigMcpApiKey))
	assert.Equal(t, "API-AGENT1", viper.GetString(constants.ConfigMcpApiKey))
	assert.Equal(t, []string{"API-AGENT1"}, fake.mcpKeys())
	assert.Contains(t, log, "Created AI agent API key apikeys-1")

	tools, err := listTools(context.Background(), session)
	require.NoError(t, err)
	assert.Len(t, tools, 4)
}

func TestConnect_UsesSavedKey(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	_, log, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.NoError(t, err)

	assert.Equal(t, 0, fake.createdKeyCount())
	assert.Equal(t, []string{"API-SAVED"}, fake.mcpKeys())
	assert.Empty(t, log)
}

func TestConnect_ReplacesRejectedSavedKey(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	viper.Set(constants.ConfigMcpApiKey, "API-REVOKED")
	f := testutil.NewMockFactory(fake.api)

	_, log, err := connectWith(t, f)
	require.NoError(t, err)

	assert.Equal(t, 1, fake.createdKeyCount())
	assert.Equal(t, []string{"API-REVOKED", "API-AGENT1"}, fake.mcpKeys())
	assert.Equal(t, "API-AGENT1", f.ConfigProvider.Get(constants.ConfigMcpApiKey))
	assert.Contains(t, log, "The saved MCP API key was rejected by the server; creating a new one.")
}

func TestConnect_ReplacesSavedUserKey(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.userKeys["API-USER"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-USER")

	_, _, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.NoError(t, err)

	assert.Equal(t, 1, fake.createdKeyCount())
	assert.Equal(t, []string{"API-USER", "API-AGENT1"}, fake.mcpKeys())
}

func TestConnect_DoesNotReplaceRejectedKeyFromEnvironment(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	t.Setenv(constants.EnvOctopusMcpApiKey, "API-FROMENV")
	require.NoError(t, viper.BindEnv(constants.ConfigMcpApiKey, constants.EnvOctopusMcpApiKey))

	_, _, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.Error(t, err)

	assert.Equal(t, "the Octopus Server rejected the MCP API key: "+invalidKeyError, err.Error())
	assert.Equal(t, 0, fake.createdKeyCount())
	assert.Equal(t, []string{"API-FROMENV"}, fake.mcpKeys())
}

func TestConnect_UserKeyFromEnvironmentExplainsAgentKeyRequirement(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.userKeys["API-USER"] = true
	t.Setenv(constants.EnvOctopusMcpApiKey, "API-USER")
	require.NoError(t, viper.BindEnv(constants.ConfigMcpApiKey, constants.EnvOctopusMcpApiKey))

	_, _, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.Error(t, err)

	assert.Equal(t, "the Octopus Server rejected the MCP API key: "+agentRequiredError, err.Error())
	assert.Equal(t, 0, fake.createdKeyCount())
}

func TestConnect_CreatesKeyOnlyOnceWhenNewKeyIsRejected(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.rejectCreatedKeys = true

	_, _, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.Error(t, err)

	assert.Equal(t, "the Octopus Server rejected the MCP API key: "+invalidKeyError, err.Error())
	assert.Equal(t, 1, fake.createdKeyCount())
}

func TestConnect_DoesNotReplaceKeyWhenMcpIsDisabled(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	fake.mcpDisabled = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	_, _, err := connectWith(t, testutil.NewMockFactory(fake.api))
	require.Error(t, err)

	assert.NotContains(t, err.Error(), "rejected the MCP API key")
	assert.Equal(t, 0, fake.createdKeyCount())
	assert.Equal(t, []string{"API-SAVED"}, fake.mcpKeys())
}

func callTool(t *testing.T, f *testutil.MockFactory, name string, arguments map[string]any) error {
	_, err := CallTool(context.Background(), f, &apiclient.FakeRequesterContext{}, &bytes.Buffer{}, name, arguments)
	return err
}

func newSpace() *spaces.Space {
	space := spaces.NewSpace("Default")
	space.ID = "Spaces-1"
	return space
}

func TestCallTool_FillsRequiredSpaceIdFromCurrentSpace(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactoryWithSpace(fake.api, newSpace()), "find_projects", map[string]any{"PartialName": "web"})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"SpaceId": "Spaces-1", "PartialName": "web"}, fake.toolArguments)
}

func TestCallTool_FillsEverySpaceParameter(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactoryWithSpace(fake.api, newSpace()), "create_release", map[string]any{"ProjectName": "Web"})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"SpaceId": "Spaces-1", "SpaceIdOrName": "Spaces-1", "ProjectName": "Web"}, fake.toolArguments)
}

func TestCallTool_KeepsSuppliedSpaceId(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactoryWithSpace(fake.api, newSpace()), "find_projects", map[string]any{"SpaceId": "Spaces-2"})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"SpaceId": "Spaces-2"}, fake.toolArguments)
}

func TestCallTool_ErrorsWhenRequiredSpaceCannotBeResolved(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactory(fake.api), "find_projects", map[string]any{})
	require.Error(t, err)

	assert.Contains(t, err.Error(), "the MCP tool 'find_projects' needs a space")
	assert.Nil(t, fake.toolArguments)
}

func TestCallTool_LeavesOptionalSpaceIdUnsetWhenNoSpaceChosen(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactoryWithSpace(fake.api, newSpace()), "find_events", map[string]any{})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{}, fake.toolArguments)
}

func TestCallTool_FillsOptionalSpaceIdWhenSpaceChosen(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")
	viper.Set(constants.ConfigSpace, "Default")

	err := callTool(t, testutil.NewMockFactoryWithSpace(fake.api, newSpace()), "find_events", map[string]any{})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"SpaceId": "Spaces-1"}, fake.toolArguments)
}

func TestCallTool_LeavesToolsWithoutSpaceAlone(t *testing.T) {
	resetViper(t)
	fake := newFakeOctopus(t)
	fake.agentKeys["API-SAVED"] = true
	viper.Set(constants.ConfigMcpApiKey, "API-SAVED")

	err := callTool(t, testutil.NewMockFactory(fake.api), "get_current_user", map[string]any{})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{}, fake.toolArguments)
}
