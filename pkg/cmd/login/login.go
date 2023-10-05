package login

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/config/set"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

const (
	FlagServer           = "server"
	FlagApiKey           = "api-key"
	FlagServiceAccountId = "service-account-id"
	FlagIdToken          = "id-token"
)

type LoginFlags struct {
	Server           *flag.Flag[string]
	ApiKey           *flag.Flag[string]
	ServiceAccountId *flag.Flag[string]
	IdToken          *flag.Flag[string]
}

func NewLoginFlags() *LoginFlags {
	return &LoginFlags{
		Server:           flag.New[string](FlagServer, false),
		ApiKey:           flag.New[string](FlagApiKey, false),
		ServiceAccountId: flag.New[string](FlagServiceAccountId, false),
		IdToken:          flag.New[string](FlagIdToken, false),
	}
}

func NewCmdLogin(f factory.Factory) *cobra.Command {
	loginFlags := NewLoginFlags()

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Octopus",
		Long:  "Login to your Octopus server using OpenID Connect or an API key. If no arguments are provided then login will be done interactively allowing provisioning of an API key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return loginRun(cmd, f, f.IsPromptEnabled(), f.Ask, loginFlags)
		},
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&loginFlags.Server.Value, loginFlags.Server.Name, "", "", "the URL of the Octopus Server to login to")
	flags.StringVarP(&loginFlags.ApiKey.Value, loginFlags.ApiKey.Name, "", "", "an API key to login with")
	flags.StringVarP(&loginFlags.ServiceAccountId.Value, loginFlags.ServiceAccountId.Name, "", "", "the ID of the service account to login with")
	flags.StringVarP(&loginFlags.IdToken.Value, loginFlags.IdToken.Name, "", "", "the ID token from your OIDC provider to login with")
	return cmd
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

func loginRun(cmd *cobra.Command, f factory.Factory, isPromptEnabled bool, ask question.Asker, flags *LoginFlags) error {
	server := flags.Server.Value
	apiKey := flags.ApiKey.Value
	serviceAccountId := flags.ServiceAccountId.Value

	if isPromptEnabled && server == "" {
		currentServer := viper.GetString(constants.ConfigUrl)

		if err := ask(&survey.Input{
			Message: "Octopus Server URL",
			Default: currentServer,
		}, &server); err != nil {
			return err
		}

		var provisionApiKey bool

		if err := ask(&survey.Confirm{
			Message: "Create a new API key",
		}, &provisionApiKey); err != nil {
			return err
		}

		if provisionApiKey {
			provisionApiKeyUrl := fmt.Sprintf("%s/app#/users/me/apiKeys", server)
			provisionApiKeyLink := output.Bluef(provisionApiKeyUrl)
			cmd.Printf("A web browser has been opened at %s. Please create an API key and paste it here. If no web browser is available or if the web browser fails to open, please use the --server and --api-key arguments directly.", provisionApiKeyLink)
			cmd.Println()

			err := browser.OpenURL(provisionApiKeyUrl)

			if err != nil {
				return err
			}
		}

		if err := ask(&survey.Input{
			Message: "API Key",
		}, &apiKey); err != nil {
			return err
		}
	}

	if server == "" {
		return errors.New("must supply server")
	}

	if serviceAccountId == "" && apiKey == "" {
		return errors.New("must supply a service account id or api key")
	}

	if apiKey != "" {

		serverLink := output.Bluef(server)

		apiKeyCredentials, err := octopusApiClient.NewApiKey(apiKey)

		if err != nil {
			return err
		}

		err = testLogin(cmd, server, apiKeyCredentials)

		if err != nil {
			return errors.New("login unsuccessful, please check that your API key is valid")
		}

		cmd.Printf("Configuring CLI to use API key for Octopus Server: %s", serverLink)
		cmd.Println()

		set.SetConfig(constants.ConfigUrl, server)
		set.SetConfig(constants.ConfigApiKey, apiKey)
		set.SetConfig(constants.ConfigAccessToken, "")

		cmd.Printf("Login successful, happy deployments!")
		cmd.Println()
	}

	if serviceAccountId != "" {
		idToken := flags.IdToken.Value

		if idToken == "" {
			return errors.New("must supply an id token when logging in with OpenID Connect")
		}

		serverLink := output.Bluef(server)
		serviceAccountOutput := output.Cyan(serviceAccountId)

		cmd.Printf("Logging in with OpenID Connect to %s using service account %s", serverLink, serviceAccountOutput)
		cmd.Println()

		openIdConfigurationEndpoint := fmt.Sprintf("%s/.well-known/openid-configuration", server)

		resp, err := http.Get(openIdConfigurationEndpoint)

		if err != nil {
			return err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		sb := string(body)

		var openIdConfiguration OpenIdConfigurationResponse

		err = json.Unmarshal([]byte(sb), &openIdConfiguration)

		if err != nil {
			return err
		}

		tokenExchangeData := TokenExchangeRequest{
			GrantType:        "urn:ietf:params:oauth:grant-type:token-exchange",
			Audience:         serviceAccountId,
			SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
			SubjectToken:     idToken,
		}

		tokenExchangeBody, err := json.Marshal(tokenExchangeData)

		if err != nil {
			return err
		}

		bodyReader := bytes.NewReader(tokenExchangeBody)

		resp, err = http.Post(openIdConfiguration.TokenEndpoint, "application/json", bodyReader)

		if err != nil {
			return err
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		sb = string(body)

		var tokenExchangeErrorResponse TokenExchangeErrorResponse

		err = json.Unmarshal([]byte(sb), &tokenExchangeErrorResponse)

		if err != nil {
			return err
		}

		if tokenExchangeErrorResponse.Error != "" {
			return errors.New(tokenExchangeErrorResponse.ErrorDescription)
		}

		var tokenExchangeResponse TokenExchangeResponse

		err = json.Unmarshal([]byte(sb), &tokenExchangeResponse)

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

		cmd.Printf("Access token obtained successfully via OpenID Connect, valid until %s", output.Cyan(expiryTime.Format(time.DateTime)))
		cmd.Println()

		err = testLogin(cmd, server, accessTokenCredentials)

		if err != nil {
			return errors.New("login unsuccessful using access token obtained via OpenID Connect")
		}

		cmd.Printf("Configuring CLI to use access token for Octopus Server: %s", serverLink)
		cmd.Println()

		set.SetConfig(constants.ConfigUrl, server)
		set.SetConfig(constants.ConfigAccessToken, tokenExchangeResponse.AccessToken)
		set.SetConfig(constants.ConfigApiKey, "")

		cmd.Printf("Login successful, happy deployments!")
		cmd.Println()
	}

	return nil
}

func testLogin(cmd *cobra.Command, server string, credentials octopusApiClient.ICredential) error {
	serverLink := output.Blue(server)

	cmd.Printf("Testing login to Octopus Server: %s", serverLink)
	cmd.Println()

	askProvider := question.NewAskProvider(survey.AskOne)

	httpClient := &http.Client{
		Transport: apiclient.NewSpinnerRoundTripper(),
	}

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
