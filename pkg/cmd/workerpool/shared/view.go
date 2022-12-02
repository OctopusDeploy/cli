package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
)

type ContributeDetailsCallback func(opts *ViewOptions, workerPool workerpools.IWorkerPool) ([]*output.DataRow, error)

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

func ViewRun(opts *ViewOptions, contributeDetails ContributeDetailsCallback) error {
	var workerPool, err = opts.Client.WorkerPools.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	data := []*output.DataRow{}

	data = append(data, output.NewDataRow("Name", fmt.Sprintf("%s %s", output.Bold(workerPool.GetName()), output.Dimf("(%s)", workerPool.GetID()))))
	data = append(data, output.NewDataRow("Worker Pool Type", getWorkerPoolTypeDescription(workerPool.GetWorkerPoolType())))
	if workerPool.GetIsDefault() {
		data = append(data, output.NewDataRow("Default", output.Green("Yes")))
	}

	if contributeDetails != nil {
		newRows, err := contributeDetails(opts, workerPool)
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
	machinescommon.DoWebForWorkerPools(workerPool, *opts.Dependencies, opts.WebFlags)
	return nil

	return nil
}

func getWorkerPoolTypeDescription(poolType workerpools.WorkerPoolType) string {
	if poolType == workerpools.WorkerPoolTypeDynamic {
		return "Dynamic"
	}

	return "Static"
}
