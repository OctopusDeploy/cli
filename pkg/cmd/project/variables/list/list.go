package list

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	variableShared "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
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
}

func NewListOptions(flags *ListFlags, dependencies *cmd.Dependencies, cmd *cobra.Command) *ListOptions {
	return &ListOptions{
		ListFlags:         flags,
		Command:           cmd,
		Dependencies:      dependencies,
		VariableCallbacks: sharedVariable.NewVariableCallbacks(dependencies),
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project variables",
		Long:  "List project variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable list "Deploy Website"
			$ %[1]s project variable list -p "Deploy Website" --git-ref refs/heads/main
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
	flags.StringVarP(&listFlags.GitRef.Value, listFlags.GitRef.Name, "", "", "The GitRef for the Config-As-Code branch")
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "The project")

	return cmd
}

type VariableAsJson struct {
	*variables.Variable
	Scope variables.VariableScopeValues
}

func listRun(opts *ListOptions) error {
	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	var allVariables []*variables.Variable
	vars, err := opts.GetProjectVariables(project.GetID())
	if err != nil {
		return err
	}
	for _, v := range vars.Variables {
		allVariables = append(allVariables, v)
	}

	if opts.GitRef.Value != "" {
		gitVars, err := opts.GetProjectVariablesByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value)
		if err != nil {
			return err
		}
		for _, v := range gitVars.Variables {
			allVariables = append(allVariables, v)
		}
	}

	sort.SliceStable(allVariables, func(i, j int) bool {
		return allVariables[i].Name < allVariables[j].Name
	})

	return output.PrintArray(allVariables, opts.Command, output.Mappers[*variables.Variable]{
		Json: func(v *variables.Variable) any {
			enhancedScope, err := variableShared.ToScopeValues(v, vars.ScopeValues)
			if err != nil {
				return err
			}
			return VariableAsJson{
				Variable: v,
				Scope:    *enhancedScope}
		},
		Table: output.TableDefinition[*variables.Variable]{
			Header: []string{"NAME", "DESCRIPTION", "VALUE", "IS PROMPTED", "ID"},
			Row: func(v *variables.Variable) []string {
				return []string{output.Bold(v.Name), v.Description, getValue(v), strconv.FormatBool(v.Prompt != nil), output.Dim(v.GetID())}
			},
		},
		Basic: func(v *variables.Variable) string {
			return v.Name
		},
	})
}

func getValue(v *variables.Variable) string {
	if v.IsSensitive {
		return "***"
	}

	return v.Value
}
