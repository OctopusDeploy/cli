// Package accesstokens mints short-lived Octopus credentials for a component
// that has to register itself with Octopus from inside a cluster.
//
// The Kubernetes agent registers itself, which means an Octopus credential has
// to reach the cluster. An access token is the least dangerous one available:
// it lasts about an hour, which is long enough to register and too short to be
// worth stealing, so no long-lived credential of the user's is left behind.
package accesstokens

import (
	"errors"
	"fmt"
	"time"

	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
)

const path = "/api/users/access-token"

// Token is a bearer token for the signed-in user.
type Token struct {
	Value string
	// Expires is zero when the token does not say when it expires.
	Expires time.Time
}

// Describe reports how long the token is good for, for a review screen that
// must not show the token itself.
func (t Token) Describe() string {
	if t.Expires.IsZero() {
		return "short-lived access token for your Octopus user"
	}
	return fmt.Sprintf("access token for your Octopus user, expires %s", t.Expires.Local().Format("15:04"))
}

// Generate asks Octopus for an access token for the signed-in user.
func Generate(client newclient.Client) (Token, error) {
	response, err := newclient.Post[struct {
		AccessToken string `json:"AccessToken"`
	}](client.HttpSession(), path, struct{}{})
	if err != nil {
		return Token{}, fmt.Errorf("could not get an access token from Octopus: %w", err)
	}
	if response.AccessToken == "" {
		return Token{}, errors.New("Octopus returned an empty access token")
	}

	return Token{Value: response.AccessToken, Expires: expiry(response.AccessToken)}, nil
}

// expiry reads the exp claim without verifying the token, which only Octopus
// can do. A token that cannot be read is still usable, so this reports no
// expiry rather than an error.
func expiry(token string) time.Time {
	var claims struct {
		Expires int64 `json:"exp"`
	}
	if err := util.DecodeJWTClaims(token, &claims); err != nil || claims.Expires <= 0 {
		return time.Time{}
	}
	return time.Unix(claims.Expires, 0)
}
