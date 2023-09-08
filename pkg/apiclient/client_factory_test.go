package apiclient_test

import (
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
	err := apiclient.ValidateMandatoryEnvironment("", apiKey, accessToken)

	assert.Error(t, err)
}

func TestValidateMandatoryEnvironment_WhenApiKeyAndAccessTokenAreNotSupplied_ReturnsError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, "", "")

	assert.Error(t, err)
}

func TestValidateMandatoryEnvironment_WhenHostAndApiKeyAreSupplied_DoesNotReturnError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, apiKey, "")

	assert.Nil(t, err)
}

func TestValidateMandatoryEnvironment_WhenHostAndAccessTokenAreSupplied_DoesNotReturnError(t *testing.T) {
	err := apiclient.ValidateMandatoryEnvironment(hostUrl, "", accessToken)

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
