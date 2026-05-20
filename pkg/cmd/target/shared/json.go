package shared

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type DeploymentTargetAsJson struct {
	Id                 string            `json:"Id"`
	Name               string            `json:"Name"`
	HealthStatus       string            `json:"HealthStatus"`
	StatusSummary      string            `json:"StatusSummary"`
	CommunicationStyle string            `json:"CommunicationStyle"`
	Environments       []string          `json:"Environments"`
	Roles              []string          `json:"Roles"`
	Tenants            []string          `json:"Tenants"`
	TenantTags         []string          `json:"TenantTags"`
	EndpointDetails    map[string]string `json:"EndpointDetails"`
	WebUrl             string            `json:"WebUrl"`
}

type DeploymentTargetWithWorkerPool struct {
	DeploymentTargetAsJson
	DefaultWorkerPool string `json:"DefaultWorkerPool"`
}

func GetDeploymentTargetAsJson(deps *cmd.Dependencies, target *machines.DeploymentTarget) any {
	environmentMap, _ := GetEnvironmentMap(deps.Client)
	tenantMap, _ := GetTenantMap(deps.Client)

	environments := resolveValues(target.EnvironmentIDs, environmentMap)
	tenants := resolveValues(target.TenantIDs, tenantMap)

	endpointDetails := GetEndpointDetails(target)

	targetJson := DeploymentTargetAsJson{
		Id:                 target.GetID(),
		Name:               target.Name,
		HealthStatus:       target.HealthStatus,
		StatusSummary:      target.StatusSummary,
		CommunicationStyle: target.Endpoint.GetCommunicationStyle(),
		Environments:       environments,
		Roles:              target.Roles,
		Tenants:            tenants,
		TenantTags:         target.TenantTags,
		EndpointDetails:    endpointDetails,
		WebUrl:             util.GenerateWebURL(deps.Host, target.SpaceID, fmt.Sprintf("infrastructure/machines/%s/settings", target.GetID())),
	}

	if workerEndpoint, ok := target.Endpoint.(machines.IRunsOnAWorker); ok {
		return DeploymentTargetWithWorkerPool{
			DeploymentTargetAsJson: targetJson,
			DefaultWorkerPool:      workerEndpoint.GetDefaultWorkerPoolID(),
		}
	}

	return targetJson
}
