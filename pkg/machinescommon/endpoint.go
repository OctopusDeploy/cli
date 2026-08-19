package machinescommon

import (
	"fmt"
	"net/url"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

// Displayed wherever the API or the SDK left us nothing to report.
const UnknownValue = "Unknown"

// Nouns for the machine kinds the endpoint helpers report on.
const (
	DeploymentTargetNoun = "deployment target"
	WorkerNoun           = "worker"
)

// Returns an empty string when the SDK left us no endpoint to read the style from.
func GetCommunicationStyle(endpoint machines.IEndpoint) string {
	if machines.IsNil(endpoint) {
		return ""
	}

	return endpoint.GetCommunicationStyle()
}

// Names a communication style for display: its friendly name where we have one,
// the raw style where we do not, and "Unknown" when there is no endpoint to
// read a style from at all.
func DescribeCommunicationStyle(endpoint machines.IEndpoint, names map[string]string) string {
	communicationStyle := GetCommunicationStyle(endpoint)
	if communicationStyle == "" {
		return UnknownValue
	}

	if name, ok := names[communicationStyle]; ok {
		return name
	}

	return communicationStyle
}

func FormatUri(uri *url.URL) string {
	if uri == nil {
		return UnknownValue
	}

	return uri.String()
}

// The API omits the version for a Tentacle it has never contacted.
func FormatTentacleVersion(details *machines.TentacleVersionDetails) string {
	if details == nil {
		return UnknownValue
	}

	return details.Version
}

func EndpointAs[T machines.IEndpoint](endpoint machines.IEndpoint, noun string, description string) (T, error) {
	typedEndpoint, ok := endpoint.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("this %s is not of type %s", noun, description)
	}

	return typedEndpoint, nil
}
