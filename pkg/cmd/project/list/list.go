package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/shared"
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
			%[1]s project list
			%[1]s project ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f)
		},
	}

	return cmd
}

type ProjectAsJson struct {
	Id                     string   `json:"Id"`
	Name                   string   `json:"Name"`
	Description            string   `json:"Description"`
	ProjectTags            []string `json:"ProjectTags,omitempty"`
	Slug                   string   `json:"Slug"`
	SpaceId                string   `json:"SpaceId"`
	ProjectGroupId         string   `json:"ProjectGroupId"`
	ProjectGroupName       string   `json:"ProjectGroupName,omitempty"`
	LifecycleId            string   `json:"LifecycleId"`
	LifecycleName          string   `json:"LifecycleName,omitempty"`
	IsDisabled             bool     `json:"IsDisabled"`
	IsVersionControlled    bool     `json:"IsVersionControlled"`
	TenantedDeploymentMode string   `json:"TenantedDeploymentMode"`
}

func listRun(cmd *cobra.Command, f factory.Factory) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	allProjects, err := client.Projects.GetAll()
	if err != nil {
		return err
	}

	// Two lookups for the whole list rather than one per project, and best-effort
	// as channel list is: listing still works without access to either. Basic
	// output only prints names, so don't pay for the round trips there.
	var lifecycleMap, projectGroupMap map[string]string
	if output.ResolveOutputFormat(cmd) != constants.OutputFormatBasic {
		lifecycleMap = shared.GetLifecycleMap(client)
		projectGroupMap = shared.GetProjectGroupMap(client)
	}

	return output.PrintArray(allProjects, cmd, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectAsJson{
				Id:                     p.GetID(),
				Name:                   p.GetName(),
				Description:            p.Description,
				ProjectTags:            p.ProjectTags,
				Slug:                   p.Slug,
				SpaceId:                p.SpaceID,
				ProjectGroupId:         p.ProjectGroupID,
				ProjectGroupName:       projectGroupMap[p.ProjectGroupID],
				LifecycleId:            p.LifecycleID,
				LifecycleName:          lifecycleMap[p.LifecycleID],
				IsDisabled:             p.IsDisabled,
				IsVersionControlled:    p.IsVersionControlled,
				TenantedDeploymentMode: shared.TenantedDeploymentMode(p),
			}
		},
		Table: output.TableDefinition[*projects.Project]{
			Header: []string{"NAME", "SLUG", "PROJECT GROUP", "LIFECYCLE", "DESCRIPTION", "TAGS"},
			Row: func(p *projects.Project) []string {
				return []string{
					output.Bold(p.Name),
					p.Slug,
					shared.DisplayName(p.ProjectGroupID, projectGroupMap[p.ProjectGroupID]),
					shared.DisplayName(p.LifecycleID, lifecycleMap[p.LifecycleID]),
					p.Description,
					output.FormatAsList(p.ProjectTags),
				}
			},
		},
		Basic: func(p *projects.Project) string {
			return p.GetName()
		},
	})
}
