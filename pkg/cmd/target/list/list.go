package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strings"
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
		Short: "List deployment targets in an instance of Octopus Deploy",
		Long:  "List deployment targets in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target list
		`), constants.ExecutableName),
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

	type TargetAsJson struct {
		Id           string   `json:"Id"`
		Name         string   `json:"Name"`
		Type         string   `json:"Type"`
		Roles        []string `json:"Roles"`
		Environments []Entity `json:"Environments"`
		Tenants      []Entity `json:"Tenants"`
		TenantTags   []string `json:"TenantTags"`
	}

	environmentMap, err := GetEnvironmentMap(opts, err)
	if err != nil {
		return err
	}

	tenantMap, err := GetTenantMap(opts, err)
	if err != nil {
		return err
	}

	return output.PrintArray(allTargets, opts.Command, output.Mappers[*machines.DeploymentTarget]{
		Json: func(item *machines.DeploymentTarget) any {
			environments := resolveEntities(item.EnvironmentIDs, environmentMap)
			tenants := resolveEntities(item.TenantIDs, tenantMap)
			return TargetAsJson{
				Id:           item.GetID(),
				Name:         item.Name,
				Type:         shared.CommunicationStyleToDeploymentTargetTypeMap[item.Endpoint.GetCommunicationStyle()],
				Roles:        item.Roles,
				Environments: environments,
				Tenants:      tenants,
				TenantTags:   item.TenantTags,
			}
		},
		Table: output.TableDefinition[*machines.DeploymentTarget]{
			Header: []string{"NAME", "TYPE", "ROLES", "ENVIRONMENTS", "TENANTS", "TAGS"},
			Row: func(item *machines.DeploymentTarget) []string {
				environmentNames := resolveValues(item.EnvironmentIDs, environmentMap)
				tenantNames := resolveValues(item.TenantIDs, tenantMap)
				return []string{output.Bold(item.Name), shared.CommunicationStyleToDescriptionMap[item.Endpoint.GetCommunicationStyle()], formatAsList(item.Roles), formatAsList(environmentNames), formatAsList(tenantNames), formatAsList(item.TenantTags)}
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

func resolveEntities(keys []string, lookup map[string]string) []Entity {
	var entities []Entity
	for _, k := range keys {
		entities = append(entities, Entity{Id: k, Name: lookup[k]})
	}

	return entities
}

func GetEnvironmentMap(opts *ListOptions, err error) (map[string]string, error) {
	environmentMap := make(map[string]string)
	allEnvs, err := opts.Client.Environments.GetAll()
	if err != nil {
		return nil, err
	}
	for _, e := range allEnvs {
		environmentMap[e.GetID()] = e.GetName()
	}
	return environmentMap, nil
}

func GetTenantMap(opts *ListOptions, err error) (map[string]string, error) {
	tenantMap := make(map[string]string)
	allEnvs, err := opts.Client.Tenants.GetAll()
	if err != nil {
		return nil, err
	}
	for _, e := range allEnvs {
		tenantMap[e.GetID()] = e.Name
	}
	return tenantMap, nil
}

func formatAsList(items []string) string {
	return strings.Join(items, ",")
}
