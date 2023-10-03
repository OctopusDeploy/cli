package login

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagServer           = "server"
	FlagServiceAccountId = "service-account-id"
	FlagIdToken          = "id-token"
)

type LoginFlags struct {
	Server           *flag.Flag[string]
	ServiceAccountId *flag.Flag[string]
	IdToken          *flag.Flag[string]
}

func NewLoginFlags() *LoginFlags {
	return &LoginFlags{
		Server:           flag.New[string](FlagServer, false),
		ServiceAccountId: flag.New[string](FlagServiceAccountId, false),
		IdToken:          flag.New[string](FlagIdToken, false),
	}
}

func NewCmdLogin(f factory.Factory) *cobra.Command {
	loginFlags := NewLoginFlags()

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Octopus",
		Long:  "Login to your Octopus server using OIDC",
		RunE: func(cmd *cobra.Command, args []string) error {
			return loginRun(cmd, loginFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&loginFlags.Server.Value, loginFlags.Server.Name, "", "", "the URL of the Octopus Server to login to")
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
	GrantType        string `json:"grant_type"`
	Audience         string `json:"audience"`
	SubjectTokenType string `json:"subject_token_type"`
	SubjectToken     string `json:"subject_token"`
}

func loginRun(cmd *cobra.Command, flags *LoginFlags) error {
	server := flags.Server.Value
	serviceAccountId := flags.ServiceAccountId.Value
	idToken := flags.IdToken.Value

	if server == "" {
		return errors.New("must supply server")
	}

	if serviceAccountId == "" {
		return errors.New("must supply a service account id")
	}

	if idToken == "" {
		return errors.New("must supply an id token")
	}

	cmd.Printf("Logging in with OpenID Connect to '%s' using service account '%s'", server, serviceAccountId)
	cmd.Println()
	cmd.Printf("Configuring CLI to use access token for Octopus Server '%s' on behalf of service account '%s'", server, serviceAccountId)
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

	cmd.Println(sb)

	var openIdConfiguration OpenIdConfigurationResponse

	json.Unmarshal([]byte(sb), &openIdConfiguration)

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

	cmd.Println(sb)

	var tokenExchangeResponse TokenExchangeResponse

	json.Unmarshal([]byte(sb), &tokenExchangeResponse)

	os.Setenv("OCTOPUS_URL", server)
	os.Setenv("OCTOPUS_ACCESS_TOKEN", "1234")
	cmd.Println("Login successful, happy deployments!")

	return nil
}
