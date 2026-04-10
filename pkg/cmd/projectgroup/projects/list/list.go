package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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

type ListOptions struct {
	*ListFlags
	Command *cobra.Command
	*cmd.Dependencies
}

func NewListOptions(flags *ListFlags, command *cobra.Command, dependencies *cmd.Dependencies) *ListOptions {
	return &ListOptions{
		ListFlags:    flags,
		Command:      command,
		Dependencies: dependencies,
	}
}

type ProjectListAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects in a project group",
		Long:  "List all projects in a project group in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project-group projects list --group "Group Name"
			$ %[1]s project-group projects ls -g "Group Name"
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewListOptions(listFlags, c, cmd.NewDependencies(f, c))
			return listRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Group.Value, "group", "g", "filter packages to match only ones that contain the given string", "")
	return cmd

}

func listRun(opts *ListOptions) error {
	groupIdOrName := opts.Group.Value

	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(groupIdOrName)
	if err != nil {
		return err
	}
	groupProjects, err := opts.Client.ProjectGroups.GetProjects(projectGroup)
	if err != nil {
		return err
	}

	return output.PrintArray(groupProjects, opts.Command, output.Mappers[*projects.Project]{
		Json: func(p *projects.Project) any {
			return ProjectListAsJson{
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
