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
	Command  *cobra.Command
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
	project, err := opts.Client.Projects.GetByIdentifier(opts.idOrName)
	if err != nil {
		return err
	}

	return output.PrintResource(project, opts.Command, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			cacBranch := "Not version controlled"
			if p.IsVersionControlled {
				cacBranch = p.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
			}
			
			return ProjectAsJson{
				Id:                    p.GetID(),
				Name:                  p.Name,
				Slug:                  p.Slug,
				Description:           p.Description,
				IsVersionControlled:   p.IsVersionControlled,
				VersionControlBranch:  cacBranch,
				WebUrl:               opts.Host + p.Links["Web"],
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "SLUG", "DESCRIPTION", "VERSION CONTROL", "WEB URL"},
			Row: func(p *projects.Project) []string {
				description := p.Description
				if description == "" {
					description = constants.NoDescription
				}
				
				cacBranch := "Not version controlled"
				if p.IsVersionControlled {
					cacBranch = p.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
				}
				
				return []string{
					output.Bold(p.Name),
					p.Slug,
					description,
					cacBranch,
					output.Blue(opts.Host + p.Links["Web"]),
				}
			},
		},
		Basic: func(p *projects.Project) string {
			return formatProjectForBasic(opts, p)
		},
	})
}

type ProjectAsJson struct {
	Id                   string `json:"Id"`
	Name                 string `json:"Name"`
	Slug                 string `json:"Slug"`
	Description          string `json:"Description"`
	IsVersionControlled  bool   `json:"IsVersionControlled"`
	VersionControlBranch string `json:"VersionControlBranch"`
	WebUrl              string `json:"WebUrl"`
}

func formatProjectForBasic(opts *ViewOptions, project *projects.Project) string {
	var result strings.Builder
	
	// header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(project.Name), output.Dimf("(%s)", project.Slug)))
	
	// version control branch
	cacBranch := "Not version controlled"
	if project.IsVersionControlled {
		cacBranch = project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
	}
	result.WriteString(fmt.Sprintf("Version control branch: %s\n", output.Cyan(cacBranch)))
	
	// description
	if project.Description == "" {
		result.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintln(output.Dim(project.Description)))
	}
	
	// footer with web URL
	url := opts.Host + project.Links["Web"]
	result.WriteString(fmt.Sprintf("View this project in Octopus Deploy: %s\n", output.Blue(url)))
	
	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}
	
	return result.String()
}
