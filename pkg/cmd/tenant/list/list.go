package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

type TenantAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenants",
		Long:  "List tenants in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant list
			$ %[1]s tenant ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f)
		},
	}

	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory) error {
	client, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	allTenants, err := client.Tenants.GetAll()
	if err != nil {
		return err
	}

	return output.PrintArray(allTenants, cmd, output.Mappers[*tenants.Tenant]{
		Json: func(t *tenants.Tenant) any {
			return TenantAsJson{
				Id:          t.GetID(),
				Name:        t.Name,
				Description: t.Description,
			}
		},
		Table: output.TableDefinition[*tenants.Tenant]{
			Header: []string{"NAME", "DESCRIPTION", "ID"},
			Row: func(t *tenants.Tenant) []string {
				return []string{output.Bold(t.Name), t.Description, t.GetID()}
			},
		},
		Basic: func(t *tenants.Tenant) string {
			return t.Name
		},
	})
}
