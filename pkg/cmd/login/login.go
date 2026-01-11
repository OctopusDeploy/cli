package login

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AlecAivazis/survey/v2"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

const (
	FlagServer           = "server"
	FlagApiKey           = "api-key"
	FlagServiceAccountId = "service-account-id"
	FlagIdToken          = "id-token"
	FlagIgnoreSslErrors  = "ignore-ssl-errors"
)

type LoginFlags struct {
	Server           *flag.Flag[string]
	ApiKey           *flag.Flag[string]
	ServiceAccountId *flag.Flag[string]
	IdToken          *flag.Flag[string]
	IgnoreSslErrors  *flag.Flag[bool]
}

func NewLoginFlags() *LoginFlags {
	return &LoginFlags{
		Server:           flag.New[string](FlagServer, false),
		ApiKey:           flag.New[string](FlagApiKey, false),
		ServiceAccountId: flag.New[string](FlagServiceAccountId, false),
		IdToken:          flag.New[string](FlagIdToken, false),
		IgnoreSslErrors:  flag.New[bool](FlagIgnoreSslErrors, false),
	}
}

func NewCmdLogin(f factory.Factory) *cobra.Command {
	loginFlags := NewLoginFlags()

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Octopus",
		Long:  "Login to your Octopus server using OpenID Connect (OIDC) or an API key. If no arguments are provided then login will be done interactively allowing creation of an API key.",
		Example: heredoc.Docf(`
			$ %[1]s login
			$ %[1]s login --server https://my.octopus.app --service-account-id b1a6f20f-0ec7-4e9a-938e-db800f945b37 --id-token eyJhbGciOiJQUzI1NiIs...
			$ %[1]s login --server https://my.octopus.app --api-key API-APIKEY123
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return loginRun(cmd, f, f.IsPromptEnabled(), f.Ask, loginFlags)
		},
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&loginFlags.Server.Value, loginFlags.Server.Name, "", "", "The URL of the Octopus Server to login to")
	flags.StringVarP(&loginFlags.ApiKey.Value, loginFlags.ApiKey.Name, "", "", "The API key to login with if using API keys")
	flags.StringVarP(&loginFlags.ServiceAccountId.Value, loginFlags.ServiceAccountId.Name, "", "", "The ID of the service account to login with if using OIDC")
	flags.StringVarP(&loginFlags.IdToken.Value, loginFlags.IdToken.Name, "", "", "The ID token from your OIDC provider to login with if using OIDC")
	flags.BoolVarP(&loginFlags.IgnoreSslErrors.Value, loginFlags.IgnoreSslErrors.Name, "", false, "Whether to ignore SSL errors")
	return cmd
}

type LoginInputs struct {
	server           string
	apiKey           string
	serviceAccountId string
	idToken          string
	ignoreSslErrors  bool
}

func loginRun(cmd *cobra.Command, f factory.Factory, isPromptEnabled bool, ask question.Asker, flags *LoginFlags) error {

	configProvider, err := f.GetConfigProvider()

	if err != nil {
		return err
	}

	inputs, err := getInputs(configProvider, flags, isPromptEnabled, ask, cmd)
	if err != nil {
		return err
	}

	if inputs.server == "" {
		return errors.New("must supply server")
	}

	if inputs.serviceAccountId == "" && inputs.apiKey == "" {
		return errors.New("must supply a service account id or api key")
	}

	if inputs.serviceAccountId != "" && inputs.apiKey != "" {
		return errors.New("can only login with one of service account id or api key")
	}

	httpClient, err := f.GetHttpClient()

	if err != nil {
		return err
	}

	// The http client could be nil, in which case we just use the default one from http
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	if inputs.ignoreSslErrors {
		if httpClient.Transport == nil {
			httpClient.Transport = &http.Transport{}
		}

		httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if inputs.apiKey != "" {
		err = loginWithApiKey(configProvider, httpClient, inputs.server, inputs.apiKey, cmd)
		if err != nil {
			return err
		}
	} else if inputs.serviceAccountId != "" {
		if inputs.idToken == "" {
			return errors.New("must supply an id token when logging in with OpenID Connect")
		}

		err = loginWithOpenIdConnect(configProvider, httpClient, inputs.server, inputs.serviceAccountId, inputs.idToken, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func loginWithApiKey(configProvider config.IConfigProvider, httpClient *http.Client, server string, apiKey string, cmd *cobra.Command) error {
	serverLink := output.Cyan(server)

	apiKeyCredentials, err := octopusApiClient.NewApiKey(apiKey)

	if err != nil {
		return err
	}

	err = testLogin(cmd, httpClient, server, apiKeyCredentials)

	if err != nil {
		cmd.Println(err)
		return errors.New("login unsuccessful, please check that your API key is valid")
	}

	cmd.Printf("Configuring CLI to use API key for Octopus Server: %s", serverLink)
	cmd.Println()

	configProvider.Set(constants.ConfigUrl, server)
	configProvider.Set(constants.ConfigApiKey, apiKey)
	configProvider.Set(constants.ConfigAccessToken, "")

	cmd.Printf("Login successful, happy deployments!")
	cmd.Println()
	return nil
}

type OpenIdConfigurationResponse struct {
	Issuer        string `json:"issuer"`
	TokenEndpoint string `json:"token_endpoint"`
}

type TokenExchangeRequest struct {
	GrantType        string `json:"grant_type"`
	Audience         string `json:"audience"`
	SubjectTokenType string `json:"subject_token_type"`
	SubjectToken     string `json:"subject_token"`
}

type TokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int32  `json:"expires_in"`
}

type TokenExchangeErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func loginWithOpenIdConnect(configProvider config.IConfigProvider, httpClient *http.Client, server string, serviceAccountId string, idToken string, cmd *cobra.Command) error {
	serverLink := output.Cyan(server)
	serviceAccountOutput := output.Cyan(serviceAccountId)

	cmd.Printf("Logging in with OpenID Connect to %s using service account %s", serverLink, serviceAccountOutput)
	cmd.Println()

	openIdConfiguration, err := getOpenIdConfiguration(httpClient, server)
	if err != nil {
		return err
	}

	tokenExchangeResponse, err := performTokenExchange(httpClient, serviceAccountId, idToken, openIdConfiguration)

	if err != nil {
		return err
	}

	expiresIn, err := time.ParseDuration(fmt.Sprintf("%ds", tokenExchangeResponse.ExpiresIn))

	if err != nil {
		return err
	}

	expiryTime := time.Now().Add(expiresIn)

	accessTokenCredentials, err := octopusApiClient.NewAccessToken(tokenExchangeResponse.AccessToken)

	if err != nil {
		return err
	}

	// No time.DateTime in go 1.19, when we have upgraded to 1.20+ we can change
	cmd.Printf("Access token obtained successfully via OpenID Connect, valid until %s", output.Cyan(expiryTime.Format("2006-01-02 15:04:05")))
	cmd.Println()

	err = testLogin(cmd, httpClient, server, accessTokenCredentials)

	if err != nil {
		cmd.Println(err)
		return errors.New("login unsuccessful using access token obtained via OpenID Connect")
	}

	cmd.Printf("Configuring CLI to use access token for Octopus Server: %s", serverLink)
	cmd.Println()

	configProvider.Set(constants.ConfigUrl, server)
	configProvider.Set(constants.ConfigAccessToken, tokenExchangeResponse.AccessToken)
	configProvider.Set(constants.ConfigApiKey, "")

	cmd.Printf("Login successful, happy deployments!")
	cmd.Println()
	return nil
}

func performTokenExchange(httpClient *http.Client, serviceAccountId string, idToken string, openIdConfiguration *OpenIdConfigurationResponse) (*TokenExchangeResponse, error) {
	tokenExchangeData := TokenExchangeRequest{
		GrantType:        "urn:ietf:params:oauth:grant-type:token-exchange",
		Audience:         serviceAccountId,
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		SubjectToken:     idToken,
	}

	tokenExchangeBody, err := json.Marshal(tokenExchangeData)

	if err != nil {
		return nil, err
	}

	bodyReader := bytes.NewReader(tokenExchangeBody)

	resp, err := httpClient.Post(openIdConfiguration.TokenEndpoint, "application/json", bodyReader)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	sb := string(body)

	var tokenExchangeErrorResponse TokenExchangeErrorResponse

	err = json.Unmarshal([]byte(sb), &tokenExchangeErrorResponse)

	if err != nil {
		return nil, err
	}

	if tokenExchangeErrorResponse.Error != "" {
		return nil, errors.New(tokenExchangeErrorResponse.ErrorDescription)
	}

	var tokenExchangeResponse TokenExchangeResponse

	err = json.Unmarshal([]byte(sb), &tokenExchangeResponse)

	if err != nil {
		return nil, err
	}
	return &tokenExchangeResponse, nil
}

func getOpenIdConfiguration(httpClient *http.Client, server string) (*OpenIdConfigurationResponse, error) {
	openIdConfigurationEndpoint := fmt.Sprintf("%s/.well-known/openid-configuration", server)

	resp, err := httpClient.Get(openIdConfigurationEndpoint)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	sb := string(body)

	var openIdConfiguration OpenIdConfigurationResponse

	err = json.Unmarshal([]byte(sb), &openIdConfiguration)

	if err != nil {
		return nil, err
	}
	return &openIdConfiguration, nil
}

func getInputs(configProvider config.IConfigProvider, flags *LoginFlags, isPromptEnabled bool, ask question.Asker, cmd *cobra.Command) (*LoginInputs, error) {
	server := flags.Server.Value
	apiKey := flags.ApiKey.Value
	serviceAccountId := flags.ServiceAccountId.Value
	idToken := flags.IdToken.Value
	ignoreSslErrors := flags.IgnoreSslErrors.Value

	if isPromptEnabled {
		if server == "" {
			currentServer := configProvider.Get(constants.ConfigUrl)

			if err := ask(&survey.Input{
				Message: "Octopus Server URL",
				Default: currentServer,
			}, &server, survey.WithValidator(survey.Required)); err != nil {
				return nil, err
			}
		}

		if apiKey == "" {
			var createNewApiKey bool

			if err := ask(&survey.Confirm{
				Message: "Create a new API key",
				Default: true,
			}, &createNewApiKey); err != nil {
				return nil, err
			}

			if createNewApiKey {
				createApiKeyUrl := fmt.Sprintf("%s/app#/users/me/apiKeys", server)
				createApiKeyLink := output.Cyan(createApiKeyUrl)
				cmd.Printf("A web browser has been opened at %s. Please create an API key and paste it here. If no web browser is available or if the web browser fails to open, please use the --server and --api-key arguments directly e.g. octopus login --server %s --api-key API-MYAPIKEY.", createApiKeyLink, server)
				cmd.Println()

				err := browser.OpenURL(createApiKeyUrl)

				if err != nil {
					return nil, err
				}
			}

			if err := ask(&survey.Input{
				Message: "API Key",
			}, &apiKey, survey.WithValidator(survey.Required)); err != nil {
				return nil, err
			}
		}
	}

	inputs := &LoginInputs{
		server:           server,
		apiKey:           apiKey,
		serviceAccountId: serviceAccountId,
		idToken:          idToken,
		ignoreSslErrors:  ignoreSslErrors,
	}
	return inputs, nil
}

func testLogin(cmd *cobra.Command, httpClient *http.Client, server string, credentials octopusApiClient.ICredential) error {
	serverLink := output.Cyan(server)

	cmd.Printf("Testing login to Octopus Server: %s", serverLink)
	cmd.Println()

	askProvider := question.NewAskProvider(survey.AskOne)

	clientFactory, err := apiclient.NewClientFactory(httpClient, server, credentials, "", askProvider)

	if err != nil {
		return err
	}

	octopus, err := clientFactory.GetSystemClient(apiclient.NewRequester(cmd))

	if err != nil {
		return err
	}

	_, err = octopus.Users.GetMe()

	if err != nil {
		return err
	}

	return nil
}
