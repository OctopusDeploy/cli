package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedBranches "github.com/OctopusDeploy/cli/pkg/cmd/project/branch/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
	"strconv"
)

const (
	FlagProject = "project"
	FlagGitRef  = "git-ref"
)

type ListFlags struct {
	GitRef  *flag.Flag[string]
	Project *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		GitRef:  flag.New[string](FlagGitRef, false),
		Project: flag.New[string](FlagProject, false),
	}
}

type ListOptions struct {
	*ListFlags
	Command            *cobra.Command
	GetProjectCallback shared.GetProjectCallback
	*sharedVariable.VariableCallbacks
	*cmd.Dependencies
	*sharedBranches.ProjectBranchCallbacks
}

func NewListOptions(flags *ListFlags, dependencies *cmd.Dependencies, cmd *cobra.Command) *ListOptions {
	return &ListOptions{
		ListFlags:    flags,
		Command:      cmd,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		ProjectBranchCallbacks: sharedBranches.NewProjectBranchCallbacks(dependencies),
	}
}

type BranchesAsJson struct {
	*projects.GitReference
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project branches",
		Long:  "List project branches in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project branch list "Deploy Website"
			$ %[1]s project variable ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewListOptions(listFlags, cmd.NewDependencies(f, c), c)

			if opts.Project.Value == "" {
				opts.Project.Value = args[0]
			}

			if opts.Project.Value == "" {
				return fmt.Errorf("must supply project identifier")
			}
			return listRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.GitRef.Value, listFlags.GitRef.Name, "", "", "The git-ref for the Config-As-Code branch")
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "The project")

	return cmd
}

func listRun(opts *ListOptions) error {
	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	branches, err := opts.GetAllBranchesCallback(project.GetID())
	if err != nil {
		return err
	}

	return output.PrintArray(branches, opts.Command, output.Mappers[*projects.GitReference]{
		Json: func(b *projects.GitReference) any {
			return BranchesAsJson{
				GitReference: b,
			}
		},
		Table: output.TableDefinition[*projects.GitReference]{
			Header: []string{"NAME", "CANONICAL NAME", "IS PROTECTED"},
			Row: func(b *projects.GitReference) []string {
				return []string{output.Bold(b.Name), b.CanonicalName, strconv.FormatBool(b.IsProtected)}
			},
		},
		Basic: func(b *projects.GitReference) string {
			return b.Name
		},
	})
}
