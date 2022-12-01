package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type DataRow struct {
	Name  string
	Value string
}

func NewDataRow(name string, value string) *DataRow {
	return &DataRow{
		Name:  name,
		Value: value,
	}
}

type ContributeEndpointCallback func(opts *ViewOptions, endpoint machines.IEndpoint) ([]*DataRow, error)

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
	var target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	data := []*DataRow{}

	data = append(data, NewDataRow("Name", fmt.Sprintf("%s %s", output.Bold(target.Name), output.Dimf("(%s)", target.GetID()))))
	data = append(data, NewDataRow("Health status", getHealthStatus(target)))
	data = append(data, NewDataRow("Current status", target.StatusSummary))

	if contributeEndpoint != nil {
		newRows, err := contributeEndpoint(opts, target.Endpoint)
		if err != nil {
			return err
		}
		for _, r := range newRows {
			data = append(data, r)
		}
	}

	environmentMap, err := GetEnvironmentMap(opts)
	if err != nil {
		return err
	}
	environmentNames := resolveValues(target.EnvironmentIDs, environmentMap)

	data = append(data, NewDataRow("Environments", output.FormatAsList(environmentNames)))
	data = append(data, NewDataRow("Roles", output.FormatAsList(target.Roles)))

	if !util.Empty(target.TenantIDs) {
		tenantMap, err := GetTenantMap(opts)
		if err != nil {
			return err
		}

		tenantNames := resolveValues(target.TenantIDs, tenantMap)
		data = append(data, NewDataRow("Tenants", output.FormatAsList(tenantNames)))
	} else {
		data = append(data, NewDataRow("Tenants", "None"))
	}

	if !util.Empty(target.TenantTags) {
		data = append(data, NewDataRow("Tenant Tags", output.FormatAsList(target.TenantTags)))
	} else {
		data = append(data, NewDataRow("Tenant Tags", "None"))
	}

	t := output.NewTable(opts.Out)
	for _, row := range data {
		t.AddRow(row.Name, row.Value)
	}
	t.Print()

	fmt.Fprintf(opts.Out, "\n")
	machinescommon.DoWebForTargets(target, opts.Dependencies, opts.WebFlags, description)
	return nil

	return nil
}

func ContributeProxy(opts *ViewOptions, proxyID string) ([]*DataRow, error) {
	if proxyID != "" {
		proxy, err := opts.Client.Proxies.GetById(proxyID)
		if err != nil {
			return nil, err
		}
		return []*DataRow{NewDataRow("Proxy", proxy.GetName())}, nil
	}

	return []*DataRow{NewDataRow("Proxy", "None")}, nil
}

func ContributeAccount(opts *ViewOptions, accountID string) ([]*DataRow, error) {
	account, err := opts.Client.Accounts.GetByID(accountID)
	if err != nil {
		return nil, err
	}
	data := []*DataRow{NewDataRow("Account", account.GetName())}
	return data, nil
}

func getHealthStatus(target *machines.DeploymentTarget) string {
	switch target.HealthStatus {
	case "Healthy":
		return output.Green(target.HealthStatus)
	case "Unhealthy":
		return output.Red(target.HealthStatus)
	default:
		return output.Yellow(target.HealthStatus)
	}
}

func GetEnvironmentMap(opts *ViewOptions) (map[string]string, error) {
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

func GetTenantMap(opts *ViewOptions) (map[string]string, error) {
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

func resolveValues(keys []string, lookup map[string]string) []string {
	var values []string
	for _, key := range keys {
		values = append(values, lookup[key])
	}
	return values
}
