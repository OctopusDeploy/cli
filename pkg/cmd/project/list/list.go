package list

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/spf13/cobra"
)

const (
	FlagGroup = "group"
)

type ListFlags struct {
	Group *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Group: flag.New[string](FlagGroup, false),
	}
}

type GetAllProjectsCallback func() ([]*projects.Project, error)
type GetProjectGroupCallback func(idOrName string) (*projectgroups.ProjectGroup, error)
type GetProjectsInGroupCallback func(projectGroup *projectgroups.ProjectGroup) ([]*projects.Project, error)

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long:  "List projects in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s project list
			%[1]s project list --group 'Default Project Group'
			%[1]s project ls -g ProjectGroups-1
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Group.Value, listFlags.Group.Name, "g", "", "list only the projects in this project group")

	return cmd
}

type ProjectAsJson struct {
	Id          string   `json:"Id"`
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	ProjectTags []string `json:"ProjectTags,omitempty"`
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	projectsToList, err := getProjects(
		flags.Group.Value,
		client.Projects.GetAll,
		client.ProjectGroups.GetByIDOrName,
		client.ProjectGroups.GetProjects)
	if err != nil {
		return err
	}

	return output.PrintArray(projectsToList, cmd, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectAsJson{
				Id:          p.GetID(),
				Name:        p.GetName(),
				Description: p.Description,
				ProjectTags: p.ProjectTags,
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "DESCRIPTION", "TAGS"},
			Row: func(p *projects.Project) []string {
				return []string{output.Bold(p.Name), p.Description, output.FormatAsList(p.ProjectTags)}
			},
		},
		Basic: func(p *projects.Project) string {
			return p.GetName()
		},
	})
}

// getProjects lists every project in the space, or only the projects in the
// named group when the group filter is supplied. The group is resolved rather
// than the project list filtered client side, so the server only sends back the
// projects that were asked for.
func getProjects(
	group string,
	getAllProjects GetAllProjectsCallback,
	getProjectGroup GetProjectGroupCallback,
	getProjectsInGroup GetProjectsInGroupCallback) ([]*projects.Project, error) {
	if group == "" {
		return getAllProjects()
	}

	projectGroup, err := getProjectGroup(group)
	if err != nil && !errors.Is(err, services.ErrItemNotFound) {
		return nil, err
	}
	// GetByIDOrName reports a miss as ErrItemNotFound; the nil check is defensive.
	if err != nil || projectGroup == nil {
		return nil, fmt.Errorf("cannot find a project group with name or ID of '%s'", group)
	}

	return getProjectsInGroup(projectGroup)
}
