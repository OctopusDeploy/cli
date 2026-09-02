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
	"github.com/google/uuid"
)

const template = "/api/{spaceId}/argocdgateways"

type RegisterCommand struct {
	SpaceID      string   `json:"SpaceId"`
	Name         string   `json:"Name"`
	Environments []string `json:"Environments"`
	// ClientID is how the gateway identifies itself to Octopus over gRPC.
	// Octopus requires the caller to choose it; a gateway registering itself
	// generates one the same way. Left empty, Register generates one.
	ClientID string `json:"ClientId,omitempty"`
}

// Registration is what Octopus hands back, and is everything the gateway needs
// to connect. The credential is shown once.
type Registration struct {
	ID       string
	Name     string
	ClientID string
	// AuthenticationToken is the gateway's own credential.
	AuthenticationToken string
	// CertificateThumbprint identifies the Octopus Server the gateway should
	// expect.
	CertificateThumbprint string
}

// registerResponse is the wire shape: the gateway resource itself is nested
// beside the credential, which is only ever returned from this call.
type registerResponse struct {
	Resource struct {
		ID       string `json:"Id"`
		Name     string `json:"Name"`
		ClientID string `json:"ClientId"`
	} `json:"Resource"`
	AuthenticationToken   string `json:"AuthenticationToken"`
	CertificateThumbprint string `json:"CertificateThumbprint"`
}

func Register(client newclient.Client, command RegisterCommand) (*Registration, error) {
	if strings.TrimSpace(command.Name) == "" {
		return nil, fmt.Errorf("a gateway name is required")
	}
	if command.ClientID == "" {
		command.ClientID = uuid.New().String()
	}

	path, err := expand(client, command.SpaceID)
	if err != nil {
		return nil, err
	}

	response, err := newclient.Post[registerResponse](client.HttpSession(), path, command)
	if err != nil {
		return nil, fmt.Errorf("could not register the Argo CD gateway with Octopus: %w", err)
	}
	if response.Resource.ID == "" {
		return nil, fmt.Errorf("Octopus did not return the registered gateway")
	}

	return &Registration{
		ID:                    response.Resource.ID,
		Name:                  response.Resource.Name,
		ClientID:              response.Resource.ClientID,
		AuthenticationToken:   response.AuthenticationToken,
		CertificateThumbprint: response.CertificateThumbprint,
	}, nil
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
