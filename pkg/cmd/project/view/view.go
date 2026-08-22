package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
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
			%[1]s project view 'Deploy Web App'
			%[1]s project view Projects-9000
			%[1]s project view deploy-web-app
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

	// best-effort, as channel list is: viewing still works without access to either
	lifecycleName := shared.GetLifecycleName(opts.Client, project.LifecycleID)
	projectGroupName := shared.GetProjectGroupName(opts.Client, project.ProjectGroupID)

	return output.PrintResource(project, opts.Command, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectAsJson{
				Id:                              p.GetID(),
				Name:                            p.Name,
				Slug:                            p.Slug,
				Description:                     p.Description,
				IsVersionControlled:             p.IsVersionControlled,
				VersionControlBranch:            versionControlBranch(p),
				ProjectTags:                     p.ProjectTags,
				WebUrl:                          webUrl(opts, p),
				SpaceId:                         p.SpaceID,
				IsDisabled:                      p.IsDisabled,
				ProjectGroupId:                  p.ProjectGroupID,
				ProjectGroupName:                projectGroupName,
				LifecycleId:                     p.LifecycleID,
				LifecycleName:                   lifecycleName,
				TenantedDeploymentMode:          shared.TenantedDeploymentMode(p),
				DeploymentProcessId:             p.DeploymentProcessID,
				VariableSetId:                   p.VariableSetID,
				IncludedLibraryVariableSetIds:   p.IncludedLibraryVariableSets,
				ClonedFromProjectId:             p.ClonedFromProjectID,
				AutoCreateRelease:               p.AutoCreateRelease,
				DefaultGuidedFailureMode:        p.DefaultGuidedFailureMode,
				DefaultToSkipIfAlreadyInstalled: p.DefaultToSkipIfAlreadyInstalled,
				DiscreteChannelRelease:          p.IsDiscreteChannelRelease,
				ReleaseNotesTemplate:            p.ReleaseNotesTemplate,
				VersioningStrategy:              p.VersioningStrategy,
				ProjectConnectivityPolicy:       p.ConnectivityPolicy,
				Templates:                       p.Templates,
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "SLUG", "PROJECT GROUP", "LIFECYCLE", "DESCRIPTION", "VERSION CONTROL", "TAGS", "WEB URL"},
			Row: func(p *projects.Project) []string {
				description := p.Description
				if description == "" {
					description = constants.NoDescription
				}

				return []string{
					output.Bold(p.Name),
					p.Slug,
					shared.DisplayName(p.ProjectGroupID, projectGroupName),
					shared.DisplayName(p.LifecycleID, lifecycleName),
					description,
					versionControlBranch(p),
					output.FormatAsList(p.ProjectTags),
					output.Blue(webUrl(opts, p)),
				}
			},
		},
		Basic: func(p *projects.Project) string {
			return formatProjectForBasic(opts, p, projectGroupName, lifecycleName)
		},
	})
}

type ProjectAsJson struct {
	Id                              string                                    `json:"Id"`
	Name                            string                                    `json:"Name"`
	Slug                            string                                    `json:"Slug"`
	Description                     string                                    `json:"Description"`
	IsVersionControlled             bool                                      `json:"IsVersionControlled"`
	VersionControlBranch            string                                    `json:"VersionControlBranch"`
	ProjectTags                     []string                                  `json:"ProjectTags,omitempty"`
	WebUrl                          string                                    `json:"WebUrl"`
	SpaceId                         string                                    `json:"SpaceId"`
	IsDisabled                      bool                                      `json:"IsDisabled"`
	ProjectGroupId                  string                                    `json:"ProjectGroupId"`
	ProjectGroupName                string                                    `json:"ProjectGroupName,omitempty"`
	LifecycleId                     string                                    `json:"LifecycleId"`
	LifecycleName                   string                                    `json:"LifecycleName,omitempty"`
	TenantedDeploymentMode          string                                    `json:"TenantedDeploymentMode"`
	DeploymentProcessId             string                                    `json:"DeploymentProcessId,omitempty"`
	VariableSetId                   string                                    `json:"VariableSetId,omitempty"`
	IncludedLibraryVariableSetIds   []string                                  `json:"IncludedLibraryVariableSetIds,omitempty"`
	ClonedFromProjectId             string                                    `json:"ClonedFromProjectId,omitempty"`
	AutoCreateRelease               bool                                      `json:"AutoCreateRelease"`
	DefaultGuidedFailureMode        string                                    `json:"DefaultGuidedFailureMode,omitempty"`
	DefaultToSkipIfAlreadyInstalled bool                                      `json:"DefaultToSkipIfAlreadyInstalled"`
	DiscreteChannelRelease          bool                                      `json:"DiscreteChannelRelease"`
	ReleaseNotesTemplate            string                                    `json:"ReleaseNotesTemplate,omitempty"`
	VersioningStrategy              *projects.VersioningStrategy              `json:"VersioningStrategy,omitempty"`
	ProjectConnectivityPolicy       *core.ConnectivityPolicy                  `json:"ProjectConnectivityPolicy,omitempty"`
	Templates                       []actiontemplates.ActionTemplateParameter `json:"Templates,omitempty"`
}

func versionControlBranch(project *projects.Project) string {
	if !project.IsVersionControlled {
		return "Not version controlled"
	}
	return project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
}

func webUrl(opts *ViewOptions, project *projects.Project) string {
	return util.GenerateWebURL(opts.Host, project.SpaceID, fmt.Sprintf("projects/%s", project.GetID()))
}

func formatProjectForBasic(opts *ViewOptions, project *projects.Project, projectGroupName string, lifecycleName string) string {
	var result strings.Builder

	// header
	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(project.Name), output.Dimf("(%s)", project.Slug)))

	// where the project sits and how it releases
	result.WriteString(fmt.Sprintf("Project group: %s\n", output.Cyan(shared.DisplayName(project.ProjectGroupID, projectGroupName))))
	result.WriteString(fmt.Sprintf("Lifecycle: %s\n", output.Cyan(shared.DisplayName(project.LifecycleID, lifecycleName))))
	result.WriteString(fmt.Sprintf("Tenanted deployment mode: %s\n", output.Cyan(shared.TenantedDeploymentMode(project))))

	// version control branch
	result.WriteString(fmt.Sprintf("Version control branch: %s\n", output.Cyan(versionControlBranch(project))))

	// tags
	if len(project.ProjectTags) > 0 {
		result.WriteString(fmt.Sprintf("Tags: %s\n", output.Cyan(output.FormatAsList(project.ProjectTags))))
	}

	// description
	if project.Description == "" {
		result.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintln(output.Dim(project.Description)))
	}

	if project.IsDisabled {
		result.WriteString(fmt.Sprintln("Project is disabled"))
	} else {
		result.WriteString(fmt.Sprintln("Project is enabled"))
	}

	// footer with web URL
	url := webUrl(opts, project)
	result.WriteString(fmt.Sprintf("View this project in Octopus Deploy: %s\n", output.Blue(url)))

	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}

	return result.String()
}
