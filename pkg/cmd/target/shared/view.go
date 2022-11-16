package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"strings"
)

type ViewFlags struct {
	*WebFlags
}

type ViewOptions struct {
	*cmd.Dependencies
	IdOrName string
	*ViewFlags
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		WebFlags: NewWebFlags(),
	}
}

func NewViewOptions(viewFlags *ViewFlags, dependencies *cmd.Dependencies, args []string) *ViewOptions {
	return &ViewOptions{
		ViewFlags:    viewFlags,
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func ViewRun(opts *ViewOptions, target *machines.DeploymentTarget) error {
	fmt.Fprintf(opts.Out, "%s %s\n", output.Bold(target.Name), output.Dimf("(%s)", target.GetID()))

	healthStatus := getHealthStatus(target)
	fmt.Fprintf(opts.Out, "Health status: %s\n", healthStatus)
	fmt.Fprintf(opts.Out, "Current status: %s\n", target.StatusSummary)

	environmentMap, err := GetEnvironmentMap(opts)
	if err != nil {
		return err
	}
	environmentNames := resolveValues(target.EnvironmentIDs, environmentMap)

	fmt.Fprintf(opts.Out, "Environments: %s\n", formatAsList(environmentNames))

	fmt.Fprintf(opts.Out, "Roles: %s\n", formatAsList(target.Roles))

	if !util.Empty(target.TenantIDs) {
		tenantMap, err := GetTenantMap(opts)
		if err != nil {
			return err
		}

		tenantNames := resolveValues(target.TenantIDs, tenantMap)
		fmt.Fprintf(opts.Out, "Tenants: %s\n", formatAsList(tenantNames))
	} else {
		fmt.Fprintln(opts.Out, "Target is not scoped to any tenants")
	}

	if !util.Empty(target.TenantTags) {
		fmt.Fprintf(opts.Out, "Tenant Tags: %s\n", formatAsList(target.TenantTags))
	} else {
		fmt.Fprintln(opts.Out, "Target is not scoped to any tenant tags")
	}

	return nil
}

func ViewProxy(opts *ViewOptions, proxyID string) error {
	if proxyID != "" {
		proxy, err := opts.Client.Proxies.GetById(proxyID)
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.Out, "Proxy: %s\n", proxy.GetName())
	} else {
		fmt.Println("No proxy configured")
	}
	return nil
}

func ViewAccount(opts *ViewOptions, accountID string) error {
	account, err := opts.Client.Accounts.GetByID(accountID)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "Account: %s\n", account.GetName())
	return nil
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
