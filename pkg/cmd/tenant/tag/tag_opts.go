package tag

import (
	"fmt"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

type GetAllTagSetsCallback func() ([]*tagsets.TagSet, error)
type GetTenantCallback func(tenantIdentifier string) (*tenants.Tenant, error)
type GetTenantsCallback func() ([]*tenants.Tenant, error)

type TagOptions struct {
	*TagFlags
	*cmd.Dependencies
	GetAllTagsCallback GetAllTagSetsCallback
	GetTenantCallback  GetTenantCallback
	GetTenantsCallback GetTenantsCallback
	tenant             *tenants.Tenant
}

func NewTagOptions(tagFlags *TagFlags, dependencies *cmd.Dependencies) *TagOptions {
	return &TagOptions{
		TagFlags:           tagFlags,
		Dependencies:       dependencies,
		GetAllTagsCallback: getAllTagSetsCallback(dependencies.Client),
		GetTenantCallback:  getTenantCallback(dependencies.Client),
		GetTenantsCallback: getTenantsCallback(dependencies.Client),
		tenant:             nil,
	}
}

func (to *TagOptions) Commit() error {
	if to.tenant == nil {
		tenant, err := to.GetTenantCallback(to.Tenant.Value)
		if err != nil {
			return err
		}
		to.tenant = tenant
	}
	to.tenant.TenantTags = to.Tag.Value

	updatedTenant, err := to.Client.Tenants.Update(to.tenant)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(to.Out, "\nSuccessfully updated tenant %s (%s).\n", updatedTenant.Name, updatedTenant.ID)
	if err != nil {
		return err
	}
	return nil
}

func (to *TagOptions) GenerateAutomationCmd() {
	if !to.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(to.CmdPath, to.Tenant, to.Tag)
		fmt.Fprintf(to.Out, "%s\n", autoCmd)
	}
}

func getTenantCallback(client *client.Client) GetTenantCallback {
	return func(tenantIdentifier string) (*tenants.Tenant, error) {
		tenant, err := client.Tenants.GetByIdentifier(tenantIdentifier)
		if err != nil {
			return nil, err
		}
		return tenant, nil
	}
}

func getTenantsCallback(client *client.Client) GetTenantsCallback {
	return func() ([]*tenants.Tenant, error) {
		allTenants, err := client.Tenants.GetAll()
		if err != nil {
			return nil, err
		}
		return allTenants, nil
	}
}

func getAllTagSetsCallback(client *client.Client) GetAllTagSetsCallback {
	return func() ([]*tagsets.TagSet, error) {
		tagSets, err := client.TagSets.GetAll()
		if err != nil {
			return nil, err
		}
		return tagSets, nil
	}
}
