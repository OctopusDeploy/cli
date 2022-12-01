package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/shared"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"strings"
)

type ContributeEndpointCallback func(opts *ViewOptions, endpoint machines.IEndpoint) ([]*shared.DataRow, error)

type ViewFlags struct {
	*machinescommon.WebFlags
}

type ViewOptions struct {
	*cmd.Dependencies
	IdOrName string
	*ViewFlags
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		WebFlags: machinescommon.NewWebFlags(),
	}
}

func NewViewOptions(viewFlags *ViewFlags, dependencies *cmd.Dependencies, args []string) *ViewOptions {
	return &ViewOptions{
		ViewFlags:    viewFlags,
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func ViewRun(opts *ViewOptions, contributeEndpoint ContributeEndpointCallback, description string) error {
	var worker, err = opts.Client.Workers.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	data := []*shared.DataRow{}

	data = append(data, shared.NewDataRow("Name", fmt.Sprintf("%s %s", output.Bold(worker.Name), output.Dimf("(%s)", worker.GetID()))))
	data = append(data, shared.NewDataRow("Health status", getHealthStatus(worker)))
	data = append(data, shared.NewDataRow("Current status", worker.StatusSummary))

	workerPoolMap, err := GetWorkerPoolMap(opts)
	workerPoolNames := resolveValues(worker.WorkerPoolIDs, workerPoolMap)
	data = append(data, shared.NewDataRow("Worker Pools", formatAsList(workerPoolNames)))

	if contributeEndpoint != nil {
		newRows, err := contributeEndpoint(opts, worker.Endpoint)
		if err != nil {
			return err
		}
		for _, r := range newRows {
			data = append(data, r)
		}
	}

	t := output.NewTable(opts.Out)
	for _, row := range data {
		t.AddRow(row.Name, row.Value)
	}
	t.Print()

	fmt.Fprintf(opts.Out, "\n")
	machinescommon.DoWebForWorkers(worker, opts.Dependencies, opts.WebFlags, description)
	return nil

	return nil
}

func ContributeProxy(opts *ViewOptions, proxyID string) ([]*shared.DataRow, error) {
	if proxyID != "" {
		proxy, err := opts.Client.Proxies.GetById(proxyID)
		if err != nil {
			return nil, err
		}
		return []*shared.DataRow{shared.NewDataRow("Proxy", proxy.GetName())}, nil
	}

	return []*shared.DataRow{shared.NewDataRow("Proxy", "None")}, nil
}

func ContributeAccount(opts *ViewOptions, accountID string) ([]*shared.DataRow, error) {
	account, err := opts.Client.Accounts.GetByID(accountID)
	if err != nil {
		return nil, err
	}
	data := []*shared.DataRow{shared.NewDataRow("Account", account.GetName())}
	return data, nil
}

func getHealthStatus(worker *machines.Worker) string {
	switch worker.HealthStatus {
	case "Healthy":
		return output.Green(worker.HealthStatus)
	case "Unhealthy":
		return output.Red(worker.HealthStatus)
	default:
		return output.Yellow(worker.HealthStatus)
	}
}

func GetWorkerPoolMap(opts *ViewOptions) (map[string]string, error) {
	workerPoolMap := make(map[string]string)

	allEnvs, err := opts.Client.WorkerPools.GetAll()
	if err != nil {
		return nil, err
	}
	for _, e := range allEnvs {
		workerPoolMap[e.ID] = e.Name
	}
	return workerPoolMap, nil
}

func formatAsList(items []string) string {
	return strings.Join(items, ", ")
}

func resolveValues(keys []string, lookup map[string]string) []string {
	var values []string
	for _, key := range keys {
		values = append(values, lookup[key])
	}
	return values
}
