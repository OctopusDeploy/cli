package delete

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagId      = "id"
	FlagName    = "name"
	FlagProject = "project"
	FlagGitRef  = "git-ref"
)

type DeleteFlags struct {
	Id      *flag.Flag[string]
	Name    *flag.Flag[string]
	Project *flag.Flag[string]
	GitRef  *flag.Flag[string]
	*question.ConfirmFlags
}

type DeleteOptions struct {
	*DeleteFlags
	*cmd.Dependencies
	shared.GetProjectCallback
	*sharedVariable.VariableCallbacks
}

func NewDeleteFlags() *DeleteFlags {

	return &DeleteFlags{
		Id:           flag.New[string](FlagId, false),
		Name:         flag.New[string](FlagName, false),
		Project:      flag.New[string](FlagProject, false),
		GitRef:       flag.New[string](FlagGitRef, false),
		ConfirmFlags: question.NewConfirmFlags(),
	}
}

func NewDeleteOptions(flags *DeleteFlags, dependencies *cmd.Dependencies) *DeleteOptions {

	return &DeleteOptions{
		DeleteFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		VariableCallbacks: sharedVariable.NewVariableCallbacks(dependencies),
	}
}

func NewDeleteCmd(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name>}",
		Aliases: []string{"del", "rm", "remove"},
		Short:   "Delete a project variable",
		Long:    "Delete a project variable in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable delete --name "variable Name" --project "Deploy Site" 
			$ %[1]s project variable delete --name "variable Name" --id 26a58596-4cd9-e072-7215-7e15cb796dd2 --project "Deploy Site" --confirm 
			$ %[1]s project variable delete --name "variable Name" --project "Deploy Site" --git-ref "refs/heads/main"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewDeleteOptions(deleteFlags, cmd.NewDependencies(f, c))

			return DeleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&deleteFlags.Id.Value, deleteFlags.Id.Name, "", "The id of the specific variable value to delete")
	flags.StringVarP(&deleteFlags.Name.Value, deleteFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVarP(&deleteFlags.Project.Value, deleteFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&deleteFlags.GitRef.Value, deleteFlags.GitRef.Name, "", "", "The GitRef for the Config-As-Code branch")
	question.RegisterConfirmDeletionFlag(cmd, &deleteFlags.Confirm.Value, "project variable")

	return cmd
}

func DeleteRun(opts *DeleteOptions) error {
	if opts.Name.Value == "" {
		return fmt.Errorf("variable name is required but was not provided")
	}

	project, err := opts.Client.Projects.GetByIdentifier(opts.Project.Value)
	if err != nil {
		return err
	}

	var projectVariables *variables.VariableSet
	if project.IsVersionControlled && opts.GitRef.Value != "" {
		projectVariables, err = opts.GetProjectVariablesByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value)
	} else {
		projectVariables, err = opts.GetProjectVariables(project.GetID())
	}

	filteredVars := util.SliceFilter(
		projectVariables.Variables,
		func(variable *variables.Variable) bool {
			return strings.EqualFold(variable.Name, opts.Name.Value)
		})

	if !util.Any(filteredVars) {
		return fmt.Errorf("cannot find variable '%s'", opts.Name.Value)
	}

	if len(filteredVars) == 0 {
		return fmt.Errorf("cannot find variable named '%s'", opts.Name.Value)
	}

	if len(filteredVars) > 1 {
		if opts.Id.Value == "" {
			return fmt.Errorf("'%s' has multiple values, supply '%s' flag", filteredVars[0].Name, FlagId)
		}

		filteredVars = util.SliceFilter(filteredVars, func(variable *variables.Variable) bool {
			return variable.ID == opts.Id.Value
		})
	}

	if len(filteredVars) == 1 {
		targetVar := filteredVars[0]
		targetIndex := -1
		for i, v := range projectVariables.Variables {
			if v.ID == targetVar.ID {
				targetIndex = i
			}
		}

		projectVariables.Variables = util.RemoveIndex(projectVariables.Variables, targetIndex)

		if opts.ConfirmFlags.Confirm.Value {
			return updateProjectVariables(opts, project, projectVariables)
		} else {
			return question.DeleteWithConfirmation(opts.Ask, "variable", targetVar.Name, targetVar.ID, func() error {
				return updateProjectVariables(opts, project, projectVariables)
			})
		}
	}

	return nil
}

func updateProjectVariables(opts *DeleteOptions, project *projects.Project, projectVariables *variables.VariableSet) error {
	if opts.GitRef.Value != "" {
		_, err := opts.Client.ProjectVariables.UpdateByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value, projectVariables)
		return err
	} else {
		return delete(opts, project, *projectVariables)
	}
}

func delete(opts *DeleteOptions, project *projects.Project, allVars variables.VariableSet) error {
	_, err := opts.Client.Variables.Update(project.GetID(), allVars)
	return err
}
