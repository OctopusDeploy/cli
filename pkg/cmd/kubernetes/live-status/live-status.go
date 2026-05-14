package livestatus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

const (
	FlagProject     = "project"
	FlagEnvironment = "environment"
	FlagTenant      = "tenant"
	FlagSummaryOnly = "summary-only"
)

type LiveStatusFlags struct {
	Project     *flag.Flag[string]
	Environment *flag.Flag[string]
	Tenant      *flag.Flag[string]
	SummaryOnly *flag.Flag[bool]
}

func NewLiveStatusFlags() *LiveStatusFlags {
	return &LiveStatusFlags{
		Project:     flag.New[string](FlagProject, false),
		Environment: flag.New[string](FlagEnvironment, false),
		Tenant:      flag.New[string](FlagTenant, false),
		SummaryOnly: flag.New[bool](FlagSummaryOnly, false),
	}
}

// API response types

type LiveStatusResponse struct {
	MachineStatuses []MachineStatus `json:"MachineStatuses"`
	Summary         StatusSummary   `json:"Summary"`
}

type MachineStatus struct {
	MachineId string                         `json:"MachineId"`
	Status    string                         `json:"Status"`
	Resources []KubernetesLiveStatusResource `json:"Resources"`
}

type KubernetesLiveStatusResource struct {
	Name                string                         `json:"Name"`
	Namespace           string                         `json:"Namespace,omitempty"`
	Kind                string                         `json:"Kind"`
	Group               string                         `json:"Group"`
	HealthStatus        string                         `json:"HealthStatus"`
	SyncStatus          string                         `json:"SyncStatus,omitempty"`
	HealthStatusMessage string                         `json:"HealthStatusMessage,omitempty"`
	SyncStatusMessage   string                         `json:"SyncStatusMessage,omitempty"`
	ResourceSourceId    string                         `json:"ResourceSourceId"`
	SourceType          string                         `json:"SourceType"`
	Children            []KubernetesLiveStatusResource `json:"Children"`
	LastUpdated         string                         `json:"LastUpdated"`
}

type StatusSummary struct {
	Status       string `json:"Status"`
	HealthStatus string `json:"HealthStatus"`
	SyncStatus   string `json:"SyncStatus"`
	LastUpdated  string `json:"LastUpdated"`
}

// FlatResource is a flattened representation of a resource in the tree, used for table output.
type FlatResource struct {
	Depth    int
	Resource KubernetesLiveStatusResource
}

func NewCmdLiveStatus(f factory.Factory) *cobra.Command {
	flags := NewLiveStatusFlags()

	cmd := &cobra.Command{
		Use:   "live-status",
		Short: "Get Kubernetes live object status",
		Long:  "Get the live status of Kubernetes resources for a project and environment in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s kubernetes live-status --project MyProject --environment Production
			$ %[1]s kubernetes live-status --project MyProject --environment Production --tenant MyTenant
			$ %[1]s kubernetes live-status --project MyProject --environment Production --summary-only
			$ %[1]s kubernetes live-status --project MyProject --environment Production -f json
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return liveStatusRun(cmd, f, flags)
		},
	}

	cmdFlags := cmd.Flags()
	cmdFlags.StringVarP(&flags.Project.Value, flags.Project.Name, "p", "", "Name or ID of the project")
	cmdFlags.StringVarP(&flags.Environment.Value, flags.Environment.Name, "e", "", "Name or ID of the environment")
	cmdFlags.StringVarP(&flags.Tenant.Value, flags.Tenant.Name, "t", "", "Name or ID of the tenant (for tenanted deployments)")
	cmdFlags.BoolVar(&flags.SummaryOnly.Value, flags.SummaryOnly.Name, false, "Return summary status only")

	return cmd
}

func liveStatusRun(cmd *cobra.Command, f factory.Factory, flags *LiveStatusFlags) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	// Resolve project
	projectId := flags.Project.Value
	if projectId == "" {
		if !f.IsPromptEnabled() {
			return errors.New("project must be specified; use --project flag or run in interactive mode")
		}
		selectedProject, err := selectors.Project("Select a project", client, f.Ask)
		if err != nil {
			return err
		}
		projectId = selectedProject.GetID()
	} else {
		resolvedProject, err := selectors.FindProject(client, projectId)
		if err != nil {
			return err
		}
		projectId = resolvedProject.GetID()
	}

	// Resolve environment
	environmentId := flags.Environment.Value
	if environmentId == "" {
		if !f.IsPromptEnabled() {
			return errors.New("environment must be specified; use --environment flag or run in interactive mode")
		}
		selectedEnvironment, err := selectors.EnvironmentSelect(f.Ask, func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(client)
		}, "Select an environment")
		if err != nil {
			return err
		}
		environmentId = selectedEnvironment.GetID()
	} else {
		resolvedEnvironment, err := selectors.FindEnvironment(client, environmentId)
		if err != nil {
			return err
		}
		environmentId = resolvedEnvironment.GetID()
	}

	// Resolve tenant (optional)
	var tenantId string
	if flags.Tenant.Value != "" {
		resolvedTenant, err := client.Tenants.GetByIdentifier(flags.Tenant.Value)
		if err != nil {
			return fmt.Errorf("failed to resolve tenant: %w", err)
		}
		tenantId = resolvedTenant.GetID()
	}

	// Build API URL
	spaceId := client.GetSpaceID()
	var apiPath string
	if tenantId != "" {
		apiPath = fmt.Sprintf("/api/%s/projects/%s/environments/%s/tenants/%s/livestatus", spaceId, projectId, environmentId, tenantId)
	} else {
		apiPath = fmt.Sprintf("/api/%s/projects/%s/environments/%s/untenanted/livestatus", spaceId, projectId, environmentId)
	}
	if flags.SummaryOnly.Value {
		apiPath += "?summaryOnly=true"
	}

	// Make API request
	req, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		return err
	}

	resp, err := client.HttpSession().DoRawRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var response LiveStatusResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Format output
	outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)

	if strings.EqualFold(outputFormat, constants.OutputFormatJson) {
		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return err
		}
		cmd.Println(string(data))
		return nil
	}

	if flags.SummaryOnly.Value {
		return printSummary(cmd, &response.Summary)
	}

	return printFullStatus(cmd, &response)
}

func printSummary(cmd *cobra.Command, summary *StatusSummary) error {
	rows := []*output.DataRow{
		output.NewDataRow("Status", summary.Status),
		output.NewDataRow("Health Status", summary.HealthStatus),
		output.NewDataRow("Sync Status", summary.SyncStatus),
		output.NewDataRow("Last Updated", summary.LastUpdated),
	}
	output.PrintRows(rows, cmd.OutOrStdout())
	return nil
}

func printFullStatus(cmd *cobra.Command, response *LiveStatusResponse) error {
	var allFlat []FlatResource
	for _, machine := range response.MachineStatuses {
		// Insert machine/gateway as a top-level grouping node
		allFlat = append(allFlat, FlatResource{
			Depth: 0,
			Resource: KubernetesLiveStatusResource{
				Name:         machine.MachineId,
				Kind:         "Machine",
				HealthStatus: machine.Status,
			},
		})
		allFlat = append(allFlat, flattenResources(machine.Resources, 1)...)
	}

	if len(allFlat) == 0 {
		cmd.Println("No Kubernetes resources found.")
		return nil
	}

	outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)
	if strings.EqualFold(outputFormat, constants.OutputFormatBasic) {
		for _, fr := range allFlat {
			indent := strings.Repeat("  ", fr.Depth)
			r := fr.Resource
			syncInfo := ""
			if r.SyncStatus != "" {
				syncInfo = fmt.Sprintf(", Sync: %s", r.SyncStatus)
			}
			cmd.Printf("%s%s (%s) - Health: %s%s\n", indent, r.Name, r.Kind, r.HealthStatus, syncInfo)
		}
		return nil
	}

	// Table format
	return output.PrintArray(allFlat, cmd, output.Mappers[FlatResource]{
		Json: func(fr FlatResource) any {
			return fr.Resource
		},
		Table: output.TableDefinition[FlatResource]{
			Header: []string{"Name", "Kind", "Namespace", "Health", "Sync", "Last Updated"},
			Row: func(fr FlatResource) []string {
				indent := strings.Repeat("  ", fr.Depth)
				return []string{
					indent + fr.Resource.Name,
					fr.Resource.Kind,
					fr.Resource.Namespace,
					fr.Resource.HealthStatus,
					fr.Resource.SyncStatus,
					fr.Resource.LastUpdated,
				}
			},
		},
		Basic: func(fr FlatResource) string {
			indent := strings.Repeat("  ", fr.Depth)
			r := fr.Resource
			return fmt.Sprintf("%s%s (%s) - Health: %s", indent, r.Name, r.Kind, r.HealthStatus)
		},
	})
}

func flattenResources(resources []KubernetesLiveStatusResource, depth int) []FlatResource {
	var result []FlatResource
	for _, r := range resources {
		result = append(result, FlatResource{Depth: depth, Resource: r})
		if len(r.Children) > 0 {
			result = append(result, flattenResources(r.Children, depth+1)...)
		}
	}
	return result
}
