package create

import (
	"fmt"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

type GetAllTagSetsCallback func() ([]*tagsets.TagSet, error)

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetAllTagsCallback GetAllTagSetsCallback
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:        createFlags,
		Dependencies:       dependencies,
		GetAllTagsCallback: getAllTagSetsCallback(dependencies.Client),
	}
}

func (co *CreateOptions) Commit() error {

	tenant := tenants.NewTenant(co.Name.Value)
	tenant.Description = co.Description.Value
	tenant.TenantTags = co.Tag.Value

	createdTenant, err := co.Client.Tenants.Add(tenant)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created tenant %s (%s).\n", createdTenant.Name, createdTenant.ID)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/tenants/%s/overview", co.Host, co.Space.GetID(), createdTenant.GetID())
	fmt.Fprintf(co.Out, "View this tenant on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description, co.Tag)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
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
