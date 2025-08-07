package view

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a deployment target",
		Long:  "View a deployment target in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s deployment-target view Machines-100
			$ %[1]s deployment-target view 'web-server'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args, c))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	// Use basic format as default for deployment target view when no -f flag is specified
	if !opts.Command.Flags().Changed(constants.FlagOutputFormat) {
		opts.Command.Flags().Set(constants.FlagOutputFormat, constants.OutputFormatBasic)
	}

	return output.PrintResource(target, opts.Command, output.Mappers[*machines.DeploymentTarget]{
		Json: func(t *machines.DeploymentTarget) any {
			return getDeploymentTargetAsJson(opts, t)
		},
		Table: output.TableDefinition[*machines.DeploymentTarget]{
			Header: []string{"NAME", "TYPE", "HEALTH", "ENVIRONMENTS", "ROLES", "TENANTS", "TENANT TAGS", "ENDPOINT DETAILS"},
			Row: func(t *machines.DeploymentTarget) []string {
				return getDeploymentTargetAsTableRow(opts, t)
			},
		},
		Basic: func(t *machines.DeploymentTarget) string {
			return getDeploymentTargetAsBasic(opts, t)
		},
	})
}

type DeploymentTargetAsJson struct {
	Id                   string            `json:"Id"`
	Name                 string            `json:"Name"`
	HealthStatus         string            `json:"HealthStatus"`
	StatusSummary        string            `json:"StatusSummary"`
	CommunicationStyle   string            `json:"CommunicationStyle"`
	Environments         []string          `json:"Environments"`
	Roles                []string          `json:"Roles"`
	Tenants              []string          `json:"Tenants"`
	TenantTags           []string          `json:"TenantTags"`
	EndpointDetails      map[string]string `json:"EndpointDetails"`
	WebUrl               string            `json:"WebUrl"`
}

func getDeploymentTargetAsJson(opts *shared.ViewOptions, target *machines.DeploymentTarget) DeploymentTargetAsJson {
	environmentMap, _ := shared.GetEnvironmentMap(opts)
	tenantMap, _ := shared.GetTenantMap(opts)
	
	environments := resolveValues(target.EnvironmentIDs, environmentMap)
	tenants := resolveValues(target.TenantIDs, tenantMap)
	
	endpointDetails := getEndpointDetails(target)
	
	return DeploymentTargetAsJson{
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
		WebUrl:             util.GenerateWebURL(opts.Host, target.SpaceID, fmt.Sprintf("infrastructure/machines/%s/settings", target.GetID())),
	}
}

func getDeploymentTargetAsTableRow(opts *shared.ViewOptions, target *machines.DeploymentTarget) []string {
	environmentMap, _ := shared.GetEnvironmentMap(opts)
	environments := resolveValues(target.EnvironmentIDs, environmentMap)
	
	healthStatus := getHealthStatusFormatted(target.HealthStatus)
	targetType := getTargetTypeDisplayName(target.Endpoint.GetCommunicationStyle())
	
	// Handle tenants
	tenants := "None"
	if !util.Empty(target.TenantIDs) {
		tenantMap, _ := shared.GetTenantMap(opts)
		tenantNames := resolveValues(target.TenantIDs, tenantMap)
		tenants = strings.Join(tenantNames, ", ")
	}
	
	// Handle tenant tags
	tenantTags := "None"
	if !util.Empty(target.TenantTags) {
		tenantTags = strings.Join(target.TenantTags, ", ")
	}
	
	// Handle endpoint details
	endpointDetails := getEndpointDetails(target)
	var endpointDetailsStr strings.Builder
	first := true
	for key, value := range endpointDetails {
		if !first {
			endpointDetailsStr.WriteString("; ")
		}
		endpointDetailsStr.WriteString(fmt.Sprintf("%s: %s", key, value))
		first = false
	}
	endpointDetailsString := endpointDetailsStr.String()
	if endpointDetailsString == "" {
		endpointDetailsString = "-"
	}
	
	return []string{
		output.Bold(target.Name),
		targetType,
		healthStatus,
		strings.Join(environments, ", "),
		strings.Join(target.Roles, ", "),
		tenants,
		tenantTags,
		endpointDetailsString,
	}
}

func getHealthStatusFormatted(status string) string {
	switch status {
	case "Healthy":
		return output.Green(status)
	case "Unhealthy":
		return output.Red(status)
	default:
		return output.Yellow(status)
	}
}

func getTargetTypeDisplayName(communicationStyle string) string {
	switch communicationStyle {
	case "None":
		return "Cloud Region"
	case "TentaclePassive":
		return "Listening Tentacle"
	case "TentacleActive":
		return "Polling Tentacle"
	case "Ssh":
		return "SSH"
	case "OfflineDrop":
		return "Offline Drop"
	case "AzureWebApp":
		return "Azure Web App"
	case "AzureCloudService":
		return "Azure Cloud Service"
	case "AzureServiceFabricCluster":
		return "Azure Service Fabric"
	case "Kubernetes":
		return "Kubernetes"
	case "StepPackage":
		return "Step Package"
	default:
		return communicationStyle
	}
}

func getDeploymentTargetAsBasic(opts *shared.ViewOptions, target *machines.DeploymentTarget) string {
	var result strings.Builder
	
	// Header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(target.Name), output.Dimf("(%s)", target.GetID())))
	
	// Health status
	healthStatus := getHealthStatusFormatted(target.HealthStatus)
	result.WriteString(fmt.Sprintf("Health status: %s\n", healthStatus))
	
	// Current status
	result.WriteString(fmt.Sprintf("Current status: %s\n", target.StatusSummary))
	
	// Target type and endpoint details
	targetType := getTargetTypeDisplayName(target.Endpoint.GetCommunicationStyle())
	result.WriteString(fmt.Sprintf("Type: %s\n", output.Cyan(targetType)))
	
	// Add endpoint-specific details
	endpointDetails := getEndpointDetails(target)
	for key, value := range endpointDetails {
		result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	
	// Environments
	environmentMap, _ := shared.GetEnvironmentMap(opts)
	environments := resolveValues(target.EnvironmentIDs, environmentMap)
	result.WriteString(fmt.Sprintf("Environments: %s\n", output.FormatAsList(environments)))
	
	// Roles
	result.WriteString(fmt.Sprintf("Roles: %s\n", output.FormatAsList(target.Roles)))
	
	// Tenants
	if !util.Empty(target.TenantIDs) {
		tenantMap, _ := shared.GetTenantMap(opts)
		tenants := resolveValues(target.TenantIDs, tenantMap)
		result.WriteString(fmt.Sprintf("Tenants: %s\n", output.FormatAsList(tenants)))
	} else {
		result.WriteString("Tenants: None\n")
	}
	
	// Tenant Tags
	if !util.Empty(target.TenantTags) {
		result.WriteString(fmt.Sprintf("Tenant Tags: %s\n", output.FormatAsList(target.TenantTags)))
	} else {
		result.WriteString("Tenant Tags: None\n")
	}
	
	// Web URL
	url := util.GenerateWebURL(opts.Host, target.SpaceID, fmt.Sprintf("infrastructure/machines/%s/settings", target.GetID()))
	result.WriteString(fmt.Sprintf("\nView this deployment target in Octopus Deploy: %s\n", output.Blue(url)))
	
	// Handle web flag
	if opts.WebFlags != nil && opts.WebFlags.Web.Value {
		machinescommon.DoWebForTargets(target, opts.Dependencies, opts.WebFlags, targetType)
	}
	
	return result.String()
}

func resolveValues(keys []string, lookup map[string]string) []string {
	var values []string
	for _, key := range keys {
		if value, exists := lookup[key]; exists {
			values = append(values, value)
		} else {
			values = append(values, key)
		}
	}
	return values
}

func getEndpointDetails(target *machines.DeploymentTarget) map[string]string {
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
