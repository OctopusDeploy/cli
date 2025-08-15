package view

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
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
		Short: "View a worker",
		Long:  "View a worker in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker view Machines-100
			$ %[1]s worker view 'worker'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args, c))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var worker, err = opts.Client.Workers.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	err = output.PrintResource(worker, opts.Command, output.Mappers[*machines.Worker]{
		Json: func(w *machines.Worker) any {
			return getWorkerAsJson(opts, w)
		},
		Table: output.TableDefinition[*machines.Worker]{
			Header: []string{"NAME", "TYPE", "HEALTH", "STATUS", "WORKER POOLS", "ENDPOINT DETAILS"},
			Row: func(w *machines.Worker) []string {
				return getWorkerAsTableRow(opts, w)
			},
		},
		Basic: func(w *machines.Worker) string {
			return getWorkerAsBasic(opts, w)
		},
	})
	if err != nil {
		return err
	}

	if opts.WebFlags != nil && opts.WebFlags.Web.Value {
		machinescommon.DoWebForWorkers(worker, opts.Dependencies, opts.WebFlags, getWorkerTypeDisplayName(worker.Endpoint.GetCommunicationStyle()))
	}

	return nil
}

type WorkerAsJson struct {
	Id                 string            `json:"Id"`
	Name               string            `json:"Name"`
	HealthStatus       string            `json:"HealthStatus"`
	StatusSummary      string            `json:"StatusSummary"`
	CommunicationStyle string            `json:"CommunicationStyle"`
	WorkerPools        []string          `json:"WorkerPools"`
	EndpointDetails    map[string]string `json:"EndpointDetails"`
	WebUrl             string            `json:"WebUrl"`
}

func getWorkerAsJson(opts *shared.ViewOptions, worker *machines.Worker) WorkerAsJson {
	workerPoolMap, _ := shared.GetWorkerPoolMap(opts)
	workerPoolNames := resolveValues(worker.WorkerPoolIDs, workerPoolMap)

	endpointDetails := getEndpointDetails(worker)

	return WorkerAsJson{
		Id:                 worker.GetID(),
		Name:               worker.Name,
		HealthStatus:       worker.HealthStatus,
		StatusSummary:      worker.StatusSummary,
		CommunicationStyle: worker.Endpoint.GetCommunicationStyle(),
		WorkerPools:        workerPoolNames,
		EndpointDetails:    endpointDetails,
		WebUrl:             util.GenerateWebURL(opts.Host, worker.SpaceID, fmt.Sprintf("infrastructure/workers/%s/settings", worker.GetID())),
	}
}

func getWorkerAsTableRow(opts *shared.ViewOptions, worker *machines.Worker) []string {
	workerPoolMap, _ := shared.GetWorkerPoolMap(opts)
	workerPoolNames := resolveValues(worker.WorkerPoolIDs, workerPoolMap)

	endpointDetails := getEndpointDetails(worker)
	endpointDetailsStr := formatEndpointDetailsForTable(endpointDetails)

	return []string{
		worker.Name,
		getWorkerTypeDisplayName(worker.Endpoint.GetCommunicationStyle()),
		getHealthStatusFormatted(worker.HealthStatus),
		worker.StatusSummary,
		strings.Join(workerPoolNames, ", "),
		endpointDetailsStr,
	}
}

func getWorkerAsBasic(opts *shared.ViewOptions, worker *machines.Worker) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(worker.Name), output.Dimf("(%s)", worker.GetID())))
	result.WriteString(fmt.Sprintf("Health status: %s\n", getHealthStatusFormatted(worker.HealthStatus)))
	result.WriteString(fmt.Sprintf("Current status: %s\n", worker.StatusSummary))
	result.WriteString(fmt.Sprintf("Communication style: %s\n", getWorkerTypeDisplayName(worker.Endpoint.GetCommunicationStyle())))

	workerPoolMap, _ := shared.GetWorkerPoolMap(opts)
	workerPoolNames := resolveValues(worker.WorkerPoolIDs, workerPoolMap)
	result.WriteString(fmt.Sprintf("Worker Pools: %s\n", output.FormatAsList(workerPoolNames)))

	// Add endpoint-specific details
	endpointDetails := getEndpointDetails(worker)
	for key, value := range endpointDetails {
		result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}

	// Web URL
	url := util.GenerateWebURL(opts.Host, worker.SpaceID, fmt.Sprintf("infrastructure/workers/%s/settings", worker.GetID()))
	result.WriteString(fmt.Sprintf("\nView this worker in Octopus Deploy: %s\n", output.Blue(url)))

	return result.String()
}

func getHealthStatusFormatted(status string) string {
	switch status {
	case "Healthy":
		return output.Green(status)
	case "Unhealthy":
		return output.Red(status)
	case "HasWarnings":
		return output.Yellow("Has Warnings")
	case "Unavailable":
		return output.Dim(status)
	default:
		return status
	}
}

func getWorkerTypeDisplayName(communicationStyle string) string {
	switch communicationStyle {
	case "TentaclePassive":
		return "Listening Tentacle"
	case "TentacleActive":
		return "Polling Tentacle"
	case "Ssh":
		return "SSH"
	default:
		return communicationStyle
	}
}

func getEndpointDetails(worker *machines.Worker) map[string]string {
	details := make(map[string]string)

	switch endpoint := worker.Endpoint.(type) {
	case *machines.ListeningTentacleEndpoint:
		details["URI"] = endpoint.URI.String()
		if endpoint.TentacleVersionDetails != nil {
			details["Tentacle version"] = endpoint.TentacleVersionDetails.Version
		}
	case *machines.PollingTentacleEndpoint:
		if endpoint.TentacleVersionDetails != nil {
			details["Tentacle version"] = endpoint.TentacleVersionDetails.Version
		}
	case *machines.SSHEndpoint:
		details["URI"] = endpoint.URI.String()
		if endpoint.DotNetCorePlatform != "" {
			details["Platform"] = endpoint.DotNetCorePlatform
		}
	}

	return details
}

func formatEndpointDetailsForTable(details map[string]string) string {
	var parts []string
	for key, value := range details {
		parts = append(parts, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.Join(parts, "; ")
}

func resolveValues(keys []string, lookup map[string]string) []string {
	var result []string
	for _, key := range keys {
		if value, exists := lookup[key]; exists {
			result = append(result, value)
		} else {
			result = append(result, key)
		}
	}
	return result
}
