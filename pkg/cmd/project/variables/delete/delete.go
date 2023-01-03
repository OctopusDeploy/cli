package delete

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
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
)

type DeleteFlags struct {
	Id      *flag.Flag[string]
	Name    *flag.Flag[string]
	Project *flag.Flag[string]
	*question.ConfirmFlags
}

type DeleteOptions struct {
	*DeleteFlags
	*cmd.Dependencies
	shared.GetProjectCallback
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		Id:           flag.New[string](FlagId, false),
		Name:         flag.New[string](FlagName, false),
		Project:      flag.New[string](FlagProject, false),
		ConfirmFlags: question.NewConfirmFlags(),
	}
}

func NewDeleteOptions(flags *DeleteFlags, dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		DeleteFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
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
			$ %[1]s project variable delete "Database Name" --project "Deploy Site" 
			$ %[1]s project variable delete "Database Name" --id 26a58596-4cd9-e072-7215-7e15cb796dd2 --project "Deploy Site" --confirm 
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewDeleteOptions(deleteFlags, cmd.NewDependencies(f, c))

			return deleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&deleteFlags.Id.Value, deleteFlags.Id.Name, "", "The id of the specific variable value to delete")
	flags.StringVarP(&deleteFlags.Name.Value, deleteFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVarP(&deleteFlags.Project.Value, deleteFlags.Project.Name, "p", "", "The project")
	question.RegisterConfirmDeletionFlag(cmd, &deleteFlags.Confirm.Value, "project variable")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if opts.Name.Value == "" {
		return fmt.Errorf("variable name is required but was not provided")
	}

	project, err := opts.Client.Projects.GetByIdentifier(opts.Project.Value)
	if err != nil {
		return err
	}

	allVars, err := opts.Client.Variables.GetAll(project.GetID())
	if err != nil {
		return err
	}

	filteredVars := util.SliceFilter(
		allVars.Variables,
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
		for i, v := range allVars.Variables {
			if v.ID == targetVar.ID {
				targetIndex = i
			}
		}

		allVars.Variables = util.RemoveIndex(allVars.Variables, targetIndex)
		if opts.ConfirmFlags.Confirm.Value {
			delete(opts, project, allVars)
		} else {
			return question.DeleteWithConfirmation(opts.Ask, "variable", targetVar.Name, targetVar.ID, func() error {
				return delete(opts, project, allVars)
			})
		}

	}

	return nil
}

func delete(opts *DeleteOptions, project *projects.Project, allVars variables.VariableSet) error {
	_, err := opts.Client.Variables.Update(project.GetID(), allVars)
	return err
}
