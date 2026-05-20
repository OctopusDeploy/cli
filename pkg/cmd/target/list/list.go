package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
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
	*shared.GetTargetsOptions
}

type Entity struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

func NewListOptions(dependencies *cmd.Dependencies, command *cobra.Command, query machines.MachinesQuery) *ListOptions {
	return &ListOptions{
		Command:           command,
		Dependencies:      dependencies,
		GetTargetsOptions: shared.NewGetTargetsOptions(dependencies, query),
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployment targets",
		Long:  "List deployment targets in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s deployment-target list
			%[1]s deployment-target ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			return ListRun(NewListOptions(cmd.NewDependencies(f, c), c, machines.MachinesQuery{}))
		},
	}

	return cmd
}

func ListRun(opts *ListOptions) error {
	allTargets, err := opts.GetTargetsCallback()
	if err != nil {
		return err
	}

	environmentMap, err := shared.GetEnvironmentMap(opts.Client)
	if err != nil {
		return err
	}

	tenantMap, err := shared.GetTenantMap(opts.Client)
	if err != nil {
		return err
	}

	workerPoolMap, err := shared.GetWorkerPoolMap(opts.Client)
	if err != nil {
		return err
	}

	return output.PrintArray(allTargets, opts.Command, output.Mappers[*machines.DeploymentTarget]{
		Json: func(item *machines.DeploymentTarget) any {
			return shared.GetDeploymentTargetAsJson(opts.Dependencies, item)
		},
		Table: output.TableDefinition[*machines.DeploymentTarget]{
			Header: []string{"NAME", "TYPE", "ROLES", "ENVIRONMENTS", "TENANTS", "TAGS", "DEFAULT WORKER POOL"},
			Row: func(item *machines.DeploymentTarget) []string {
				environmentNames := resolveValues(item.EnvironmentIDs, environmentMap)
				tenantNames := resolveValues(item.TenantIDs, tenantMap)
				workerPool := shared.ResolveDefaultWorkerPool(item, workerPoolMap, "None")
				return []string{output.Bold(item.Name), machinescommon.CommunicationStyleToDescriptionMap[item.Endpoint.GetCommunicationStyle()], output.FormatAsList(item.Roles), output.FormatAsList(environmentNames), output.FormatAsList(tenantNames), output.FormatAsList(item.TenantTags), workerPool}
			},
		},
		Basic: func(item *machines.DeploymentTarget) string {
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
