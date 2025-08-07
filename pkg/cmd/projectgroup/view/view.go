package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
	Command  *cobra.Command
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
				cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(opts.idOrName)
	if err != nil {
		return err
	}

	projects, err := opts.Client.ProjectGroups.GetProjects(projectGroup)
	if err != nil {
		return err
	}

	// Use basic format as default for project group view when no -f flag is specified
	if !opts.Command.Flags().Changed(constants.FlagOutputFormat) {
		opts.Command.Flags().Set(constants.FlagOutputFormat, constants.OutputFormatBasic)
	}

	return output.PrintResource(projectGroup, opts.Command, output.Mappers[*projectgroups.ProjectGroup]{
		Json: func(pg *projectgroups.ProjectGroup) any {
			projectList := make([]ProjectInfo, 0, len(projects))
			for _, project := range projects {
				projectList = append(projectList, ProjectInfo{
					Id:   project.GetID(),
					Name: project.GetName(),
					Slug: project.Slug,
				})
			}
			
			return ProjectGroupAsJson{
				Id:          pg.GetID(),
				Name:        pg.GetName(),
				Description: pg.Description,
				Projects:    projectList,
				WebUrl:      util.GenerateWebURL(opts.Host, pg.SpaceID, fmt.Sprintf("projects?projectGroupId=%s", pg.GetID())),
			}
		},
		Table: output.TableDefinition[*projectgroups.ProjectGroup]{
			Header: []string{"NAME", "DESCRIPTION", "PROJECTS COUNT", "WEB URL"},
			Row: func(pg *projectgroups.ProjectGroup) []string {
				description := pg.Description
				if description == "" {
					description = constants.NoDescription
				}
				
				url := util.GenerateWebURL(opts.Host, pg.SpaceID, fmt.Sprintf("projects?projectGroupId=%s", pg.GetID()))
				
				return []string{
					output.Bold(pg.GetName()),
					description,
					fmt.Sprintf("%d", len(projects)),
					output.Blue(url),
				}
			},
		},
		Basic: func(pg *projectgroups.ProjectGroup) string {
			return formatProjectGroupForBasic(opts, pg, projects)
		},
	})
}

type ProjectInfo struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
	Slug string `json:"Slug"`
}

type ProjectGroupAsJson struct {
	Id          string        `json:"Id"`
	Name        string        `json:"Name"`
	Description string        `json:"Description"`
	Projects    []ProjectInfo `json:"Projects"`
	WebUrl      string        `json:"WebUrl"`
}

func formatProjectGroupForBasic(opts *ViewOptions, projectGroup *projectgroups.ProjectGroup, projects []*projects.Project) string {
	var result strings.Builder
	
	// header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(projectGroup.GetName()), output.Dimf("(%s)", projectGroup.GetID())))
	
	// description
	if projectGroup.Description == "" {
		result.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintln(output.Dim(projectGroup.Description)))
	}
	
	// projects
	result.WriteString(fmt.Sprintf(output.Cyan("\nProjects:\n")))
	for _, project := range projects {
		result.WriteString(fmt.Sprintf("%s (%s)\n", output.Bold(project.GetName()), project.Slug))
	}
	
	// footer with web URL
	url := util.GenerateWebURL(opts.Host, projectGroup.SpaceID, fmt.Sprintf("projects?projectGroupId=%s", projectGroup.GetID()))
	result.WriteString(fmt.Sprintf("\nView this project group in Octopus Deploy: %s\n", output.Blue(url)))
	
	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}
	
	return result.String()
}
