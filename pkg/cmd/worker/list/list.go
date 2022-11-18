package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strings"
)

type ListOptions struct {
	*cobra.Command
	*cmd.Dependencies
	*shared.GetWorkersOptions
}

type Entity struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
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
		Short:   "List workers in an instance of Octopus Deploy",
		Long:    "List workers in an instance of Octopus Deploy.",
		Aliases: []string{"ls"},
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s workers list
		`), constants.ExecutableName),
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
		Id          string   `json:"Id"`
		Name        string   `json:"Name"`
		Type        string   `json:"Type"`
		WorkerPools []Entity `json:"WorkerPools"`
	}

	workerPoolMap, err := GetWorkerPoolMap(opts, err)
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

func resolveEntities(keys []string, lookup map[string]string) []Entity {
	var entities []Entity
	for _, k := range keys {
		entities = append(entities, Entity{Id: k, Name: lookup[k]})
	}

	return entities
}

func GetWorkerPoolMap(opts *ListOptions, err error) (map[string]string, error) {
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
