package apiclient_test

import (
	"net/http"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/viper"
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

// The code this replaced set InsecureSkipVerify on the shared http.DefaultTransport
// unconditionally, so the CLI never verified the Octopus server certificate. Verification
// is on by default now and the user has to opt out of it explicitly.
func TestNewClientFactoryFromConfig_TlsVerification(t *testing.T) {
	tests := []struct {
		name                   string
		ignoreSslErrors        any
		wantInsecureSkipVerify bool
	}{
		{name: "verifies by default", ignoreSslErrors: nil, wantInsecureSkipVerify: false},
		{name: "verifies when the opt out is false", ignoreSslErrors: false, wantInsecureSkipVerify: false},
		{name: "skips verification when opted out", ignoreSslErrors: true, wantInsecureSkipVerify: true},
		{name: "skips verification when opted out via a string", ignoreSslErrors: "true", wantInsecureSkipVerify: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Set(constants.ConfigUrl, hostUrl)
			viper.Set(constants.ConfigApiKey, apiKey)
			viper.Set(constants.ConfigIgnoreSslErrors, test.ignoreSslErrors)
			t.Cleanup(func() {
				viper.Set(constants.ConfigUrl, "")
				viper.Set(constants.ConfigApiKey, "")
				viper.Set(constants.ConfigIgnoreSslErrors, nil)
			})

			factory, err := apiclient.NewClientFactoryFromConfig(qa)
			testutil.RequireSuccess(t, err)

			httpClient, err := factory.GetHttpClient()
			testutil.RequireSuccess(t, err)

			spinnerRoundTripper, ok := httpClient.Transport.(*apiclient.SpinnerRoundTripper)
			if !assert.True(t, ok, "expected a *apiclient.SpinnerRoundTripper") {
				return
			}
			transport, ok := spinnerRoundTripper.Next.(*http.Transport)
			if !assert.True(t, ok, "expected the spinner to wrap an *http.Transport") {
				return
			}

			if test.wantInsecureSkipVerify {
				if assert.NotNil(t, transport.TLSClientConfig) {
					assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
				}
			} else if transport.TLSClientConfig != nil {
				assert.False(t, transport.TLSClientConfig.InsecureSkipVerify, "the CLI must verify the Octopus server certificate unless the user opts out")
			}
		})
	}
}
