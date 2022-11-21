package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/spf13/cobra"
)

type ProjectGroupAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project groups",
		Long:  "List project groups in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project-group list
			$ %[1]s project-group ls
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

	allProjects, err := client.ProjectGroups.GetAll()
	if err != nil {
		return err
	}

	return output.PrintArray(allProjects, cmd, output.Mappers[*projectgroups.ProjectGroup]{
		Json: func(p *projectgroups.ProjectGroup) any {
			return ProjectGroupAsJson{
				Id:          p.GetID(),
				Name:        p.Name,
				Description: p.Description,
			}
		},
		Table: output.TableDefinition[*projectgroups.ProjectGroup]{
			Header: []string{"NAME", "DESCRIPTION"},
			Row: func(p *projectgroups.ProjectGroup) []string {
				return []string{output.Bold(p.Name), p.Description}
			},
		},
		Basic: func(p *projectgroups.ProjectGroup) string {
			return p.Name
		},
	})
}
