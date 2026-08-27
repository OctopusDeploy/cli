// Package argocdgateways registers Argo CD gateways with Octopus Server.
//
// The gateway chart can register itself, but only if it is given an Octopus
// credential to do it with, which then lives in the cluster for as long as the
// gateway does. Registering from the CLI instead means only the gateway's own
// credential ever reaches the cluster.
package argocdgateways

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
)

const template = "/api/{spaceId}/argocdgateways"

// RegisterCommand asks Octopus to register a gateway.
type RegisterCommand struct {
	SpaceID      string   `json:"SpaceId"`
	Name         string   `json:"Name"`
	Environments []string `json:"Environments"`
	// ClientID re-registers an existing gateway, which is how one gateway is
	// shared across spaces.
	ClientID string `json:"ClientId,omitempty"`
	// PreserveAuthenticationToken keeps the gateway's existing credential when
	// re-registering. The response then carries no token.
	PreserveAuthenticationToken bool `json:"PreserveAuthenticationToken,omitempty"`
}

// Registration is what Octopus hands back, and is everything the gateway needs
// to connect. The credential is shown once.
type Registration struct {
	ID       string `json:"Id"`
	Name     string `json:"Name"`
	SpaceID  string `json:"SpaceId"`
	ClientID string `json:"ClientId"`
	// AuthenticationToken is empty when re-registering with
	// PreserveAuthenticationToken.
	AuthenticationToken string `json:"AuthenticationToken"`
	// CertificateThumbprint identifies the Octopus Server the gateway should
	// expect. Octopus has spelled this field both ways.
	CertificateThumbprint string `json:"CertificateThumbprint"`
	Thumbprint            string `json:"Thumbprint"`
}

// Thumb returns whichever spelling of the thumbprint the server used.
func (r Registration) Thumb() string {
	if r.CertificateThumbprint != "" {
		return r.CertificateThumbprint
	}
	return r.Thumbprint
}

func Register(client newclient.Client, command RegisterCommand) (*Registration, error) {
	if strings.TrimSpace(command.Name) == "" {
		return nil, fmt.Errorf("a gateway name is required")
	}

	path, err := expand(client, command.SpaceID)
	if err != nil {
		return nil, err
	}

	registration, err := newclient.Post[Registration](client.HttpSession(), path, command)
	if err != nil {
		return nil, fmt.Errorf("could not register the Argo CD gateway with Octopus: %w", err)
	}
	return registration, nil
}

// DeleteByID removes a registration. Used to undo one when the install that
// followed it did not work, so a failed attempt does not leave a gateway in
// Octopus that will never connect.
func DeleteByID(client newclient.Client, spaceID, id string) error {
	path, err := expand(client, spaceID)
	if err != nil {
		return err
	}
	return newclient.Delete(client.HttpSession(), path+"/"+id)
}

// List returns the gateways already registered in a space, so a name collision
// can be reported before it silently takes over an existing gateway.
func List(client newclient.Client, spaceID string) ([]Registration, error) {
	path, err := expand(client, spaceID)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []Registration `json:"Items"`
	}
	page, err := newclient.Get[struct {
		Items []Registration `json:"Items"`
	}](client.HttpSession(), path)
	if err != nil {
		return nil, fmt.Errorf("could not list the Argo CD gateways in this space: %w", err)
	}
	response = *page
	return response.Items, nil
}

func expand(client newclient.Client, spaceID string) (string, error) {
	if spaceID == "" {
		spaceID = client.GetSpaceID()
	}
	path, err := client.URITemplateCache().Expand(template, map[string]any{"spaceId": spaceID})
	if err != nil {
		return "", err
	}
	return path, nil
}
