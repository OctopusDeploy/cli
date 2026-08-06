package shared

import (
	"fmt"
	"math"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type GetTargetsCallback func() ([]*machines.DeploymentTarget, error)

type GetTargetsOptions struct {
	GetTargetsCallback
}

func NewGetTargetsOptions(dependencies *cmd.Dependencies, query machines.MachinesQuery) *GetTargetsOptions {
	return &GetTargetsOptions{
		GetTargetsCallback: func() ([]*machines.DeploymentTarget, error) {
			return GetAllTargets(*dependencies.Client, query)
		},
	}
}

func NewGetTargetsOptionsForAllTargets(dependencies *cmd.Dependencies) *GetTargetsOptions {
	return &GetTargetsOptions{
		GetTargetsCallback: func() ([]*machines.DeploymentTarget, error) {
			return GetAllTargets(*dependencies.Client, machines.MachinesQuery{})
		},
	}
}

func GetAllTargets(client client.Client, query machines.MachinesQuery) ([]*machines.DeploymentTarget, error) {
	query.Skip = 0
	query.Take = math.MaxInt32
	res, err := client.Machines.Get(query)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func GetEndpointDetails(target *machines.DeploymentTarget) map[string]string {
	details := make(map[string]string)

	switch target.Endpoint.GetCommunicationStyle() {
	case "AzureWebApp":
		if endpoint, ok := target.Endpoint.(*machines.AzureWebAppEndpoint); ok {
			webApp := endpoint.WebAppName
			if endpoint.WebAppSlotName != "" {
				webApp = fmt.Sprintf("%s/%s", webApp, endpoint.WebAppSlotName)
			}
			details["Web App"] = webApp
		}
	case "Kubernetes":
		if endpoint, ok := target.Endpoint.(*machines.KubernetesEndpoint); ok {
			details["Authentication Type"] = endpoint.Authentication.GetAuthenticationType()
		}
	case "Ssh":
		if endpoint, ok := target.Endpoint.(*machines.SSHEndpoint); ok {
			details["URI"] = endpoint.URI.String()
			runtime := "Mono"
			if endpoint.DotNetCorePlatform != "" {
				runtime = endpoint.DotNetCorePlatform
			}
			details["Runtime architecture"] = runtime
		}
	case "TentaclePassive":
		if endpoint, ok := target.Endpoint.(*machines.ListeningTentacleEndpoint); ok {
			details["URI"] = endpoint.URI.String()
			details["Tentacle version"] = endpoint.TentacleVersionDetails.Version
		}
	case "TentacleActive":
		if endpoint, ok := target.Endpoint.(*machines.PollingTentacleEndpoint); ok {
			details["URI"] = endpoint.URI.String()
			details["Tentacle version"] = endpoint.TentacleVersionDetails.Version
		}
	case "None":
		// Cloud regions typically don't have additional endpoint details
	case "OfflineDrop", "StepPackage", "AzureCloudService", "AzureServiceFabricCluster":
		// These endpoints don't have specific details we can easily extract
	}

	return details
}
