package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/model"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	ssh "github.com/OctopusDeploy/cli/pkg/cmd/worker/ssh/view"
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
		Id            string         `json:"Id"`
		Name          string         `json:"Name"`
		Type          string         `json:"Type"`
		HealthStatus  string         `json:"HealthStatus"`
		StatusSummary string         `json:"StatusSummary"`
		WorkerPools   []model.Entity `json:"WorkerPools"`
		URI           string         `json:"URI"`
		Version       string         `json:"Version,omitempty"`
		Runtime       string         `json:"Runtime,omitempty"`
		Account       *model.Entity  `json:"Account,omitempty"`
		Proxy         string         `json:"Proxy"`
	}

	workerPoolMap, err := GetWorkerPoolMap(opts)
	if err != nil {
		return err
	}

	return output.PrintArray(allTargets, opts.Command, output.Mappers[*machines.Worker]{
		Json: func(item *machines.Worker) any {

			return TargetAsJson{
				Id:            item.GetID(),
				Name:          item.Name,
				Type:          machinescommon.CommunicationStyleToDeploymentTargetTypeMap[item.Endpoint.GetCommunicationStyle()],
				HealthStatus:  item.HealthStatus,
				StatusSummary: item.StatusSummary,
				WorkerPools:   resolveEntities(item.WorkerPoolIDs, workerPoolMap),
				URI:           getEndpointUri(item.Endpoint),
				Version:       getVersion(item.Endpoint),
				Runtime:       getRuntimeArchitecture(item.Endpoint),
				Account:       getAccount(opts, item.Endpoint),
				Proxy:         getProxy(opts, item.Endpoint),
			}
		},
		Table: output.TableDefinition[*machines.Worker]{
			Header: []string{"NAME", "TYPE", "WORKER POOLS"},
			Row: func(item *machines.Worker) []string {
				poolNames := resolveValues(item.WorkerPoolIDs, workerPoolMap)
				return []string{output.Bold(item.Name), machinescommon.CommunicationStyleToDescriptionMap[item.Endpoint.GetCommunicationStyle()], output.FormatAsList(poolNames)}
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

func getEndpointUri(end machines.IEndpoint) string {
	endpointUri := ""
	switch end.GetCommunicationStyle() {
	case "TentaclePassive":
		endpoint := end.(*machines.ListeningTentacleEndpoint)
		endpointUri = endpoint.URI.String()
	case "TentacleActive":
		endpoint := end.(*machines.PollingTentacleEndpoint)
		endpointUri = endpoint.URI.String()
	case "Ssh":
		endpoint := end.(*machines.SSHEndpoint)
		endpointUri = endpoint.URI.String()
	}
	return endpointUri
}

func getVersion(end machines.IEndpoint) string {
	tentacleVersion := ""
	switch end.GetCommunicationStyle() {
	case "TentaclePassive":
		endpoint := end.(*machines.ListeningTentacleEndpoint)
		tentacleVersion = endpoint.TentacleVersionDetails.Version
	case "TentacleActive":
		endpoint := end.(*machines.PollingTentacleEndpoint)
		tentacleVersion = endpoint.TentacleVersionDetails.Version
	}
	return tentacleVersion
}

func getRuntimeArchitecture(end machines.IEndpoint) string {
	switch end.GetCommunicationStyle() {
	case "Ssh":
		endpoint := end.(*machines.SSHEndpoint)
		return ssh.GetRuntimeArchitecture(endpoint)
	}
	return ""
}

func getAccount(opts *ListOptions, end machines.IEndpoint) *model.Entity {
	accountId := ""
	switch end.GetCommunicationStyle() {
	case "Ssh":
		endpoint := end.(*machines.SSHEndpoint)
		accountId = endpoint.AccountID
	}
	if accountId != "" {
		account, err := opts.Client.Accounts.GetByID(accountId)
		if err != nil {
			return nil
		}
		entity := &model.Entity{Id: account.GetID(), Name: account.GetName()}
		return entity
	}
	return nil
}

func getProxy(opts *ListOptions, end machines.IEndpoint) string {
	proxyId := ""
	switch end.GetCommunicationStyle() {
	case "TentaclePassive":
		endpoint := end.(*machines.ListeningTentacleEndpoint)
		proxyId = endpoint.ProxyID
	case "Ssh":
		endpoint := end.(*machines.SSHEndpoint)
		proxyId = endpoint.ProxyID
	}

	if proxyId != "" {
		proxy, err := opts.Client.Proxies.GetById(proxyId)
		if err != nil {
			return "None"
		}
		return proxy.GetName()
	}
	return "None"
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
