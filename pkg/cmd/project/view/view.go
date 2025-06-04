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
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
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
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id> | <slug>}",
		Short: "View a project",
		Long:  "View a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project view 'Deploy Web App'
			$ %[1]s project view Projects-9000
			$ %[1]s project view deploy-web-app
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

type ProjectAsJson struct {
	Id                  string `json:"Id"`
	Name                string `json:"Name"`
	Description         string `json:"Description"`
	IsVersionControlled bool   `json:"IsVersionControlled"`
	Branch              string `json:"Branch"`
}

func viewRun(cmd *cobra.Command, opts *ViewOptions) error {
	project, err := opts.Client.Projects.GetByIdentifier(opts.idOrName)
	if err != nil {
		return err
	}

	cacBranch := "Not version controlled"
	if project.IsVersionControlled {
		cacBranch = project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
	}

	url := opts.Host + project.Links["Web"]

	return output.PrintResource(project, cmd, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectAsJson{
				Id:                  p.GetID(),
				Name:                p.GetName(),
				Description:         p.Description,
				IsVersionControlled: p.IsVersionControlled,
				Branch:              cacBranch,
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "DESCRIPTION", "ID", "CAC BRANCH"},
			Row: func(p *projects.Project) []string {
				return []string{output.Bold(p.Name), p.Description, output.Dim(p.GetID()), cacBranch}
			},
		},
		Basic: func(item *projects.Project) string {
			var s strings.Builder

			s.WriteString(fmt.Sprintf("%s %s\n", output.Bold(project.Name), output.Dimf("(%s)", project.Slug)))
			s.WriteString(fmt.Sprintf("Version control branch: %s\n", output.Cyan(cacBranch)))
			if project.Description == "" {
				s.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
			} else {
				s.WriteString(fmt.Sprintln(output.Dim(project.Description)))
			}

			// footer
			s.WriteString(fmt.Sprintf("View this project in Octopus Deploy: %s\n", output.Blue(url)))

			if opts.flags.Web.Value {
				browser.OpenURL(url)
			}

			return s.String()
		},
	})
}
