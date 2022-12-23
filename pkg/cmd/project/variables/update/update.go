package update

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	variableShared "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagId       = "id"
	FlagProject  = "project"
	FlagName     = "name"
	FlagValue    = "value"
	FlagUnscoped = "unscoped"
)

type UpdateFlags struct {
	Id       *flag.Flag[string]
	Project  *flag.Flag[string]
	Name     *flag.Flag[string]
	Value    *flag.Flag[string]
	Unscoped *flag.Flag[bool]

	*variableShared.ScopeFlags
}

type UpdateOptions struct {
	*UpdateFlags
	*cmd.Dependencies
	shared.GetProjectCallback
	shared.GetAllProjectsCallback
}

func NewUpdateFlags() *UpdateFlags {
	return &UpdateFlags{
		Id:         flag.New[string](FlagId, false),
		Project:    flag.New[string](FlagProject, false),
		Name:       flag.New[string](FlagName, false),
		Value:      flag.New[string](FlagValue, false),
		Unscoped:   flag.New[bool](FlagUnscoped, false),
		ScopeFlags: variableShared.NewScopeOptions(),
	}
}

func NewUpdateOptions(flags *UpdateFlags, dependencies *cmd.Dependencies) *UpdateOptions {
	return &UpdateOptions{
		UpdateFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(*dependencies.Client) },
	}
}

func NewUpdateCmd(f factory.Factory) *cobra.Command {
	updateFlags := NewUpdateFlags()
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the value of a project variable",
		Long:  "Update the value of a project variable in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable update
			$ %[1]s project variable update --name varname --value "abc"
			$ %[1]s project variable update --name varname --value "password"
			$ %[1]s project variable update --name varname --unscoped
			$ %[1]s project variable update --name varname --environment-scope test
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewUpdateOptions(updateFlags, cmd.NewDependencies(f, c))

			return updateRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&updateFlags.Id.Value, updateFlags.Id.Name, "", "The variable id to update")
	flags.StringVarP(&updateFlags.Project.Value, updateFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&updateFlags.Name.Value, updateFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVar(&updateFlags.Value.Value, updateFlags.Value.Name, "", "The value to set on the variable")
	flags.BoolVar(&updateFlags.Unscoped.Value, updateFlags.Unscoped.Name, false, "Remove all shared from the variable, cannot be used with shared")
	variableShared.RegisterScopeFlags(cmd, updateFlags.ScopeFlags)

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	if opts.Unscoped.Value && scopesProvided(opts) {
		return fmt.Errorf("cannot provide '%s' and scope flags together", opts.Unscoped.Name)
	}

	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	projectVariables, err := opts.Client.Variables.GetAll(project.GetID())
	if err != nil {
		return err
	}

	variable, err := getVariable(opts, project, projectVariables)
	if err != nil {
		return err
	}

	if variable.IsSensitive {
		opts.Value.Secure = true
	}

	updatedScope, err := variableShared.ToVariableScope(projectVariables, opts.ScopeFlags, project)
	if err != nil {
		return err
	}

	if opts.Value.Value != "" {
		variable.Value = opts.Value.Value
	}

	if opts.Unscoped.Value {
		variable.Scope = variables.VariableScope{}
	} else {
		if !updatedScope.IsEmpty() {
			variable.Scope = *updatedScope
		}
	}

	_, err = opts.Client.Variables.UpdateSingle(project.GetID(), variable)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(opts.Out, "Successfully updated variable '%s' in project '%s'\n", opts.Name.Value, project.GetName())

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Id, opts.Name, opts.Value, opts.Project, opts.EnvironmentsScopes, opts.ChannelScopes, opts.StepScopes, opts.TargetScopes, opts.TagScopes, opts.RoleScopes, opts.ProcessScopes, opts.Unscoped)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func getVariable(opts *UpdateOptions, project *projects.Project, projectVariables variables.VariableSet) (*variables.Variable, error) {
	var variable *variables.Variable
	var err error
	if opts.Id.Value != "" {
		variable, err = opts.Client.Variables.GetByID(project.GetID(), opts.Id.Value)
		if err != nil {
			return nil, err
		}

		if variable == nil {
			return nil, fmt.Errorf("cannot find variable with id '%s'", opts.Id.Value)
		}
	} else {
		variables := util.SliceFilter(projectVariables.Variables, func(v *variables.Variable) bool {
			return strings.EqualFold(v.Name, opts.Name.Value)
		})

		if len(variables) == 0 {
			return nil, fmt.Errorf("cannot find variable with name '%s'", opts.Name.Value)
		} else if len(variables) > 1 {
			return nil, fmt.Errorf("'%s' has multiple values, supply '%s' flag", variables[0].Name, FlagId)
		} else {
			variable = variables[0]
		}
	}

	return variable, err
}

func scopesProvided(opts *UpdateOptions) bool {
	return !(util.Empty(opts.EnvironmentsScopes.Value) ||
		util.Empty(opts.ChannelScopes.Value) ||
		util.Empty(opts.TagScopes.Value) ||
		util.Empty(opts.RoleScopes.Value) ||
		util.Empty(opts.StepScopes.Value) ||
		util.Empty(opts.ProcessScopes.Value) ||
		util.Empty(opts.TargetScopes.Value))
}

func PromptMissing(opts *UpdateOptions) error {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = projectSelector("You have not specified a Project. Please select one:", opts.GetAllProjectsCallback, opts.Ask)
		if err != nil {
			return nil
		}
		opts.Project.Value = project.GetName()
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
		if err != nil {
			return err
		}
	}

	projectVariables, err := opts.Client.Variables.GetAll(project.GetID())
	if err != nil {
		return err
	}

	var variable *variables.Variable
	if opts.Id.Value != "" || opts.Name.Value != "" {
		variable, err = getVariable(opts, project, projectVariables)
		if err != nil {
			variable, err = promptForVariable(opts, projectVariables)
			if err != nil {
				return err
			}
		}
		opts.Id.Value = variable.GetID()
		opts.Name.Value = variable.Name
	} else {
		variable, err = promptForVariable(opts, projectVariables)
		opts.Id.Value = variable.GetID()
		opts.Name.Value = variable.Name
	}

	if opts.Value.Value == "" {
		var updateValue bool
		opts.Ask(&survey.Confirm{
			Message: "Do you want to update the variable value?",
			Default: false,
		}, &updateValue)

		if updateValue {
			variableShared.PromptValue(opts.Ask, &opts.Value.Value, variable.Type)
		}
	}

	if !scopesProvided(opts) {
		selectedOption, err := selectors.SelectOptions(opts.Ask, "Do you want to change the variable scoping?", getScopeUpdateOptions)
		if err != nil {
			return err
		}
		switch selectedOption.Value {
		case "unscoped":
			opts.Unscoped.Value = true
		case "replace":
			variableShared.PromptScopes(opts.Ask, projectVariables, opts.ScopeFlags, variable.Prompt != nil)
		}
	}

	return nil
}

func promptForVariable(opts *UpdateOptions, projectVariables variables.VariableSet) (*variables.Variable, error) {
	selectedOption, err := selectors.Select(opts.Ask, "Select the variable you wish to update", func() ([]*variables.Variable, error) { return projectVariables.Variables, nil }, func(v *variables.Variable) string { return formatVariableSelection(v) })

	if err != nil {
		return nil, err
	}
	return selectedOption, nil
}

func formatVariableSelection(v *variables.Variable) string {
	value := v.Value
	if v.IsSensitive {
		value = "***"
	}
	if value == "" {
		value = output.Dim("(no value)")
	}

	return fmt.Sprintf("%s (%s) = %s", v.Name, output.Dim(v.GetID()), value)
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string { return p.GetName() })
}

func getScopeUpdateOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Leave", Value: "leave"},
		{Display: "Replace", Value: "replace"},
		{Display: "Unscope", Value: "unscope"},
	}
}
