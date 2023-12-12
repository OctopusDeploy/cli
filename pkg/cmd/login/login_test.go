package login_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/login"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestLogin_ApiKey(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"interactive: configures server and api key correctly", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login"})
				return rootCmd.ExecuteC()
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Octopus Server URL",
			}).AnswerWith(currentHost)

			_ = qa.ExpectQuestion(t, &survey.Confirm{
				Message: "Create a new API key",
				Default: true,
			}).AnswerWith(false)

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "API Key",
			}).AnswerWith(apiKey)

			user := users.NewUser("test", "Test")

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWith(user)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, currentHost, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Equal(t, apiKey, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"interactive: uses server if supplied", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost})
				return rootCmd.ExecuteC()
			})

			_ = qa.ExpectQuestion(t, &survey.Confirm{
				Message: "Create a new API key",
				Default: true,
			}).AnswerWith(false)

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "API Key",
			}).AnswerWith(apiKey)

			user := users.NewUser("test", "Test")

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWith(user)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, currentHost, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Equal(t, apiKey, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"interactive: uses api key if supplied", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--api-key", apiKey})
				return rootCmd.ExecuteC()
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Octopus Server URL",
			}).AnswerWith(currentHost)

			user := users.NewUser("test", "Test")

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWith(user)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, currentHost, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Equal(t, apiKey, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: uses server and api key", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--api-key", apiKey, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			user := users.NewUser("test", "Test")

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWith(user)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, currentHost, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Equal(t, apiKey, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: if server parameter not supplied returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--api-key", apiKey, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "must supply server")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: if server value not supplied returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			apiKey := "API-APIKEY01"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", "", "--api-key", apiKey, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "must supply server")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: if api key parameter not supplied returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "must supply a service account id or api key")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: if api key value not supplied returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--api-key", "", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "must supply a service account id or api key")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"non-interactive: if api key is invalid returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			apiKey := "API-APIINVALIDKEY"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--api-key", apiKey, "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWithError(errors.New("Your API key is invalid"))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "login unsuccessful, please check that your API key is valid")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactory(api)
			fac.AskProvider = askProvider
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, fac, api, qa, rootCmd, stdout, stderr)
		})
	}
}

func TestLogin_OpenIdConnect(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"logs in with OIDC, configures server and access token correctly", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			serviceAccountId := "c247db46-e32a-4906-bf51-2dff9e7431b6"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--service-account-id", serviceAccountId, "--id-token", "test", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			tokenExchangeEndpoint := "/token/v1"

			openIdDiscoveryConfiguration := login.OpenIdConfigurationResponse{
				Issuer:        currentHost,
				TokenEndpoint: tokenExchangeEndpoint,
			}

			api.ExpectRequest(t, "GET", "/.well-known/openid-configuration").RespondWith(openIdDiscoveryConfiguration)

			tokenExchangeResponse := login.TokenExchangeResponse{
				AccessToken: "accesstoken",
				ExpiresIn:   3600,
			}

			api.ExpectRequest(t, "POST", tokenExchangeEndpoint).RespondWith(tokenExchangeResponse)

			user := users.NewUser("test", "Test")

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWith(user)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, currentHost, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Equal(t, "accesstoken", fac.ConfigProvider.Get(constants.ConfigAccessToken))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
		}},

		{"when token exchange with Octopus Server fails, returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			serviceAccountId := "c247db46-e32a-4906-bf51-2dff9e7431b6"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--service-account-id", serviceAccountId, "--id-token", "test", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			tokenExchangeEndpoint := "/token/v1"

			openIdDiscoveryConfiguration := login.OpenIdConfigurationResponse{
				Issuer:        currentHost,
				TokenEndpoint: tokenExchangeEndpoint,
			}

			api.ExpectRequest(t, "GET", "/.well-known/openid-configuration").RespondWith(openIdDiscoveryConfiguration)

			tokenExchangeErrorResponse := login.TokenExchangeErrorResponse{
				Error:            "invalid_request",
				ErrorDescription: "Your request was not valid",
			}

			api.ExpectRequest(t, "POST", tokenExchangeEndpoint).RespondWith(tokenExchangeErrorResponse)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "Your request was not valid")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},

		{"when test of access token fails, returns error", func(t *testing.T, fac *testutil.MockFactory, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			currentHost := fac.GetCurrentHost()
			serviceAccountId := "c247db46-e32a-4906-bf51-2dff9e7431b6"

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"login", "--server", currentHost, "--service-account-id", serviceAccountId, "--id-token", "test", "--no-prompt"})
				return rootCmd.ExecuteC()
			})

			tokenExchangeEndpoint := "/token/v1"

			openIdDiscoveryConfiguration := login.OpenIdConfigurationResponse{
				Issuer:        currentHost,
				TokenEndpoint: tokenExchangeEndpoint,
			}

			api.ExpectRequest(t, "GET", "/.well-known/openid-configuration").RespondWith(openIdDiscoveryConfiguration)

			tokenExchangeResponse := login.TokenExchangeResponse{
				AccessToken: "accesstoken",
				ExpiresIn:   3600,
			}

			api.ExpectRequest(t, "POST", tokenExchangeEndpoint).RespondWith(tokenExchangeResponse)

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/users/me").RespondWithError(errors.New("Your access token is invalid"))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "login unsuccessful using access token obtained via OpenID Connect")

			// Check that none of the config got set
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigUrl))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigApiKey))
			assert.Empty(t, fac.ConfigProvider.Get(constants.ConfigAccessToken))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactory(api)
			fac.AskProvider = askProvider
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, fac, api, qa, rootCmd, stdout, stderr)
		})
	}
}
