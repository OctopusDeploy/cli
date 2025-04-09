package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type ViewFlags struct {
	Web *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client   *client.Client
	Host     string
	out      io.Writer
	idOrName string
	flags    *ViewFlags
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id> | <slug>}",
		Short: "View a project group",
		Long:  "View a project group in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project-group view 'Default Project Group'
			$ %[1]s project-group view ProjectGroups-9000
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				cmd.OutOrStdout(),
				args[0],
				viewFlags,
			}

			return viewRun(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

type ProjectLookup struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type ProjectGroupAsJson struct {
	Id          string             `json:"Id"`
	Name        string             `json:"Name"`
	Description string             `json:"Description"`
	Projects    []output.IdAndName `json:"Projects"`
}

func viewRun(cmd *cobra.Command, opts *ViewOptions) error {
	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(opts.idOrName)
	if err != nil {
		return err
	}

	projects, err := opts.Client.ProjectGroups.GetProjects(projectGroup)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/app#/%s/projects?projectGroupId=%s", opts.Host, projectGroup.SpaceID, projectGroup.GetID())

	return output.PrintResource(projectGroup, cmd, output.Mappers[*projectgroups.ProjectGroup]{
		Json: func(pg *projectgroups.ProjectGroup) any {
			projectsLookup := []output.IdAndName{}

			for _, p := range projects {
				projectsLookup = append(projectsLookup, output.IdAndName{Id: p.ID, Name: p.Name})
			}

			return ProjectGroupAsJson{
				Id:          pg.GetID(),
				Name:        pg.GetName(),
				Description: pg.Description,
				Projects:    projectsLookup,
			}
		},
		Table: output.TableDefinition[*projectgroups.ProjectGroup]{
			Header: []string{"NAME", "DESCRIPTION", "ID"},
			Row: func(pg *projectgroups.ProjectGroup) []string {
				return []string{output.Bold(pg.Name), pg.Description, output.Dim(pg.GetID())}
			},
		},
		Basic: func(item *projectgroups.ProjectGroup) string {
			var s strings.Builder

			s.WriteString(fmt.Sprintf("%s %s\n", output.Bold(projectGroup.GetName()), output.Dimf("(%s)", projectGroup.GetID())))
			if projectGroup.Description == "" {
				s.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
			} else {
				s.WriteString(fmt.Sprintln(output.Dim(projectGroup.Description)))
			}

			s.WriteString(fmt.Sprintf(output.Cyan("\nProjects:\n")))
			for _, project := range projects {
				s.WriteString(fmt.Sprintf("%s (%s)\n", output.Bold(project.GetName()), project.Slug))
			}

			// footer
			s.WriteString(fmt.Sprintf("\nView this project group in Octopus Deploy: %s\n", output.Blue(url)))

			if opts.flags.Web.Value {
				browser.OpenURL(url)
			}

			return s.String()
		},
	})
}
