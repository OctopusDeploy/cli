package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long:  "List projects in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project list
			$ %[1]s project ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f)
		},
	}

	return cmd
}

type ProjectAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

func listRun(cmd *cobra.Command, f factory.Factory) error {
	client, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	allProjects, err := client.Projects.GetAll()
	if err != nil {
		return err
	}

	return output.PrintArray(allProjects, cmd, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectAsJson{
				Id:          p.GetID(),
				Name:        p.GetName(),
				Description: p.Description,
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "DESCRIPTION"},
			Row: func(p *projects.Project) []string {
				return []string{output.Bold(p.Name), p.Description}
			},
		},
		Basic: func(p *projects.Project) string {
			return p.GetName()
		},
	})
}
