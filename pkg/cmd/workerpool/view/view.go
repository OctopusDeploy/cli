package view

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a worker pool",
		Long:  "View a worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker-pool view WorkerPools-3
			$ %[1]s worker-pool view 'linux workers'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ViewRun(shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args, c))
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	var workerPool, err = opts.Client.WorkerPools.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	return output.PrintResource(workerPool, opts.Command, output.Mappers[workerpools.IWorkerPool]{
		Json: func(wp workerpools.IWorkerPool) any {
			return getWorkerPoolAsJson(opts, wp)
		},
		Table: output.TableDefinition[workerpools.IWorkerPool]{
			Header: []string{"NAME", "TYPE", "DEFAULT", "WORKERS", "HEALTHY", "UNHEALTHY"},
			Row: func(wp workerpools.IWorkerPool) []string {
				return getWorkerPoolAsTableRow(opts, wp)
			},
		},
		Basic: func(wp workerpools.IWorkerPool) string {
			return getWorkerPoolAsBasic(opts, wp)
		},
	})
}

type WorkerPoolAsJson struct {
	Id              string            `json:"Id"`
	Name            string            `json:"Name"`
	WorkerPoolType  string            `json:"WorkerPoolType"`
	IsDefault       bool              `json:"IsDefault"`
	Workers         WorkerStats       `json:"Workers"`
	WorkerPoolDetails map[string]string `json:"WorkerPoolDetails"`
	WebUrl          string            `json:"WebUrl"`
}

type WorkerStats struct {
	Total              int `json:"Total"`
	Healthy            int `json:"Healthy"`
	HasWarnings        int `json:"HasWarnings"`
	Unhealthy          int `json:"Unhealthy"`
	Unavailable        int `json:"Unavailable"`
	Disabled           int `json:"Disabled"`
	SSH                int `json:"SSH"`
	ListeningTentacle  int `json:"ListeningTentacle"`
	PollingTentacle    int `json:"PollingTentacle"`
}

func getWorkerPoolAsJson(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) WorkerPoolAsJson {
	workers, _ := getWorkers(opts, workerPool)
	workerStats := calculateWorkerStats(workers)
	
	workerPoolDetails := getWorkerPoolDetails(workerPool)
	
	return WorkerPoolAsJson{
		Id:              workerPool.GetID(),
		Name:            workerPool.GetName(),
		WorkerPoolType:  getWorkerPoolTypeDescription(workerPool.GetWorkerPoolType()),
		IsDefault:       workerPool.GetIsDefault(),
		Workers:         workerStats,
		WorkerPoolDetails: workerPoolDetails,
		WebUrl:          util.GenerateWebURL(opts.Host, workerPool.GetSpaceID(), fmt.Sprintf("infrastructure/workerpools/%s", workerPool.GetID())),
	}
}

func getWorkerPoolAsTableRow(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) []string {
	workers, _ := getWorkers(opts, workerPool)
	workerStats := calculateWorkerStats(workers)
	
	defaultStatus := ""
	if workerPool.GetIsDefault() {
		defaultStatus = output.Green("Yes")
	} else {
		defaultStatus = "No"
	}
	
	return []string{
		output.Bold(workerPool.GetName()),
		getWorkerPoolTypeDescription(workerPool.GetWorkerPoolType()),
		defaultStatus,
		fmt.Sprintf("%d", workerStats.Total),
		output.Greenf("%d", workerStats.Healthy),
		output.Redf("%d", workerStats.Unhealthy+workerStats.Unavailable),
	}
}

func getWorkerPoolAsBasic(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) string {
	var result strings.Builder
	
	// Header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(workerPool.GetName()), output.Dimf("(%s)", workerPool.GetID())))
	result.WriteString(fmt.Sprintf("Worker Pool Type: %s\n", getWorkerPoolTypeDescription(workerPool.GetWorkerPoolType())))
	
	if workerPool.GetIsDefault() {
		result.WriteString(fmt.Sprintf("Default: %s\n", output.Green("Yes")))
	}
	
	// Worker statistics
	workers, _ := getWorkers(opts, workerPool)
	workerStats := calculateWorkerStats(workers)
	
	result.WriteString(fmt.Sprintf("Total workers: %d\n", workerStats.Total))
	if workerStats.Disabled > 0 {
		result.WriteString(fmt.Sprintf("Disabled workers: %s\n", output.Dimf("%d", workerStats.Disabled)))
	}
	if workerStats.Healthy > 0 {
		result.WriteString(fmt.Sprintf("Healthy workers: %s\n", output.Greenf("%d", workerStats.Healthy)))
	}
	if workerStats.HasWarnings > 0 {
		result.WriteString(fmt.Sprintf("Has Warnings workers: %s\n", output.Yellowf("%d", workerStats.HasWarnings)))
	}
	if workerStats.Unhealthy > 0 {
		result.WriteString(fmt.Sprintf("Unhealthy workers: %s\n", output.Redf("%d", workerStats.Unhealthy)))
	}
	if workerStats.Unavailable > 0 {
		result.WriteString(fmt.Sprintf("Unavailable workers: %s\n", output.Redf("%d", workerStats.Unavailable)))
	}
	
	// Worker type breakdown
	if workerStats.SSH > 0 {
		result.WriteString(fmt.Sprintf("SSH workers: %d\n", workerStats.SSH))
	}
	if workerStats.ListeningTentacle > 0 {
		result.WriteString(fmt.Sprintf("Listening Tentacle workers: %d\n", workerStats.ListeningTentacle))
	}
	if workerStats.PollingTentacle > 0 {
		result.WriteString(fmt.Sprintf("Polling Tentacle workers: %d\n", workerStats.PollingTentacle))
	}
	
	// Worker pool specific details
	workerPoolDetails := getWorkerPoolDetails(workerPool)
	for key, value := range workerPoolDetails {
		result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	
	// Web URL
	url := util.GenerateWebURL(opts.Host, workerPool.GetSpaceID(), fmt.Sprintf("infrastructure/workerpools/%s", workerPool.GetID()))
	result.WriteString(fmt.Sprintf("\nView this worker pool in Octopus Deploy: %s\n", output.Blue(url)))
	
	// Handle web flag
	if opts.WebFlags != nil && opts.WebFlags.Web.Value {
		machinescommon.DoWebForWorkerPools(workerPool, opts.Dependencies, opts.WebFlags)
	}
	
	return result.String()
}

func getWorkers(opts *shared.ViewOptions, workerPool workerpools.IWorkerPool) ([]*machines.Worker, error) {
	if workerPool.GetWorkerPoolType() == workerpools.WorkerPoolTypeStatic {
		return opts.Client.WorkerPools.GetWorkers(workerPool)
	}
	// Dynamic worker pools don't have static workers
	return []*machines.Worker{}, nil
}

func calculateWorkerStats(workers []*machines.Worker) WorkerStats {
	stats := WorkerStats{}
	
	for _, worker := range workers {
		stats.Total++
		
		if worker.IsDisabled {
			stats.Disabled++
		}
		
		switch worker.HealthStatus {
		case "Healthy":
			stats.Healthy++
		case "HasWarnings":
			stats.HasWarnings++
		case "Unhealthy":
			stats.Unhealthy++
		case "Unavailable":
			stats.Unavailable++
		}
		
		switch worker.Endpoint.GetCommunicationStyle() {
		case "Ssh":
			stats.SSH++
		case "TentaclePassive":
			stats.ListeningTentacle++
		case "TentacleActive":
			stats.PollingTentacle++
		}
	}
	
	return stats
}

func getWorkerPoolDetails(workerPool workerpools.IWorkerPool) map[string]string {
	details := make(map[string]string)
	
	if workerPool.GetWorkerPoolType() == workerpools.WorkerPoolTypeDynamic {
		dynamicPool := workerPool.(*workerpools.DynamicWorkerPool)
		details["Worker Type"] = string(dynamicPool.WorkerType)
	}
	
	return details
}

func getWorkerPoolTypeDescription(poolType workerpools.WorkerPoolType) string {
	if poolType == workerpools.WorkerPoolTypeDynamic {
		return "Dynamic"
	}
	return "Static"
}
