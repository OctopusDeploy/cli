package view

import (
	"io"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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

type ProjectsListAsJson struct {
	Name string `json:"Name"`
	Slug string `json:"Slug"`
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

			return viewRun(opts, cmd)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions, cmd *cobra.Command) error {
	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(opts.idOrName)
	if err != nil {
		return err
	}

	allProjects, err := opts.Client.ProjectGroups.GetProjects(projectGroup)
	if err != nil {
		return err
	}

	return output.PrintArray(allProjects, cmd, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectsListAsJson{
				Name: p.Name,
				Slug: p.Slug,
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"PROJECT NAME", "PROJECT SLUG"},
			Row: func(p *projects.Project) []string {
				return []string{output.Bold(p.Name), p.Slug}
			},
		},
		Basic: func(p *projects.Project) string {
			return p.Name
		},
	})
}
