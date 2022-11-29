package list

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/model"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

type ListOptions struct {
	*cobra.Command
	*cmd.Dependencies
	*shared.GetWorkersOptions
}

func NewListOptions(dependencies *cmd.Dependencies, command *cobra.Command, filter func(*machines.Worker) bool) *ListOptions {
	return &ListOptions{
		Command:           command,
		Dependencies:      dependencies,
		GetWorkersOptions: shared.NewGetWorkersOptions(dependencies, filter),
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List workers",
		Long:    "List workers in Octopus Deploy",
		Aliases: []string{"ls"},
		Example: heredoc.Docf("$ %s worker list", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ListRun(NewListOptions(cmd.NewDependencies(f, c), c, nil))
		},
	}

	return cmd
}

func ListRun(opts *ListOptions) error {
	allTargets, err := opts.GetWorkersCallback()
	if err != nil {
		return err
	}

	type TargetAsJson struct {
		Id          string         `json:"Id"`
		Name        string         `json:"Name"`
		Type        string         `json:"Type"`
		WorkerPools []model.Entity `json:"WorkerPools"`
	}

	workerPoolMap, err := GetWorkerPoolMap(opts)
	if err != nil {
		return err
	}

	return output.PrintArray(allTargets, opts.Command, output.Mappers[*machines.Worker]{
		Json: func(item *machines.Worker) any {

			return TargetAsJson{
				Id:          item.GetID(),
				Name:        item.Name,
				Type:        machinescommon.CommunicationStyleToDeploymentTargetTypeMap[item.Endpoint.GetCommunicationStyle()],
				WorkerPools: resolveEntities(item.WorkerPoolIDs, workerPoolMap),
			}
		},
		Table: output.TableDefinition[*machines.Worker]{
			Header: []string{"NAME", "TYPE", "WORKER POOLS"},
			Row: func(item *machines.Worker) []string {
				poolNames := resolveValues(item.WorkerPoolIDs, workerPoolMap)
				return []string{output.Bold(item.Name), machinescommon.CommunicationStyleToDescriptionMap[item.Endpoint.GetCommunicationStyle()], formatAsList(poolNames)}
			},
		},
		Basic: func(item *machines.Worker) string {
			return item.Name
		},
	})
}

func resolveValues(keys []string, lookup map[string]string) []string {
	var values []string
	for _, key := range keys {
		values = append(values, lookup[key])
	}
	return values
}

func resolveEntities(keys []string, lookup map[string]string) []model.Entity {
	var entities []model.Entity
	for _, k := range keys {
		entities = append(entities, model.Entity{Id: k, Name: lookup[k]})
	}

	return entities
}

func GetWorkerPoolMap(opts *ListOptions) (map[string]string, error) {
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
	return strings.Join(items, ",")
}
