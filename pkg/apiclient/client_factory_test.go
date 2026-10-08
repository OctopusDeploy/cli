package apiclient_test

import (
	"net/http"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/stretchr/testify/assert"
)

const hostUrl = "http://octopus.com"
const apiKey = "API-APIKEY01"
const accessToken = "token"

func TestValidateMandatoryEnvironment_WhenHostIsNotSupplied_ReturnsError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment("", apiKey, accessToken, false)

	assert.Error(t, err)
}

func TestValidateMandatoryEnvironment_WhenApiKeyAndAccessTokenAreNotSupplied_ReturnsError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, "", "", false)

	assert.Error(t, err)
}

func TestValidateMandatoryEnvironment_WhenHostAndApiKeyAreSupplied_DoesNotReturnError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, apiKey, "", false)

	assert.Nil(t, err)
}

func TestValidateMandatoryEnvironment_WhenHostAndAccessTokenAreSupplied_DoesNotReturnError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, "", accessToken, false)

	assert.Nil(t, err)
}

func TestNewClientFactory_WhenHostIsNotSupplied_ReturnsError(t *testing.T) {
	apiKeyCredential, _ := client.NewApiKey(apiKey)
	_, err := apiclient.NewClientFactory(nil, "", apiKeyCredential, "", qa)
	assert.Error(t, err)
}

func TestNewClientFactory_WhenHostIsNotAValidUrl_ReturnsError(t *testing.T) {
	apiKeyCredential, _ := client.NewApiKey(apiKey)
	_, err := apiclient.NewClientFactory(nil, "http_foo:bar/this-is-invalid", apiKeyCredential, "", qa)
	assert.Error(t, err)
}

func TestNewClientFactory_WhenApiKeyAndAccessTokenAreNotSupplied_ReturnsError(t *testing.T) {
	_, err := apiclient.NewClientFactory(nil, hostUrl, nil, "", qa)
	assert.Error(t, err)
}

func TestNewClientFactory_WhenHostAndApiKeyAreSupplied_ReturnsClientFactory(t *testing.T) {
	apiKeyCredential, _ := client.NewApiKey(apiKey)
	factory, err := apiclient.NewClientFactory(nil, hostUrl, apiKeyCredential, "", qa)
	testutil.RequireSuccess(t, err)
	assert.NotNil(t, factory)
}

func TestNewClientFactory_WhenHostAndAccessTokenAreSupplied_ReturnsClientFactory(t *testing.T) {
	accessTokenCredential, _ := client.NewAccessToken(accessToken)
	factory, err := apiclient.NewClientFactory(nil, hostUrl, accessTokenCredential, "", qa)
	testutil.RequireSuccess(t, err)
	assert.NotNil(t, factory)
}

func TestClientFactory_EnableDryRunGuard_RefusesMutatingRequests(t *testing.T) {
	transport := &testutil.RecordingRoundTripper{}
	apiKeyCredential, _ := client.NewApiKey(apiKey)
	clientFactory, err := apiclient.NewClientFactory(&http.Client{Transport: transport}, hostUrl, apiKeyCredential, "", qa)
	testutil.RequireSuccess(t, err)

	clientFactory.EnableDryRunGuard()

	httpClient, err := clientFactory.GetHttpClient()
	testutil.RequireSuccess(t, err)

	_, err = httpClient.Post(hostUrl+"/api/Spaces-1/releases/create/v1", "application/json", nil)
	assert.ErrorContains(t, err, "dry run blocked a POST request to /api/Spaces-1/releases/create/v1")
	assert.Empty(t, transport.Requests, "a mutating request must not reach the server")

	_, err = httpClient.Get(hostUrl + "/api/Spaces-1/projects/all")
	assert.Nil(t, err)
	assert.Len(t, transport.Requests, 1, "read-only requests still go through")
}
