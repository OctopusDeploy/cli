package set

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagProject          = "project"
	FlagName             = "name"
	FlagValue            = "value"
	FlagEnvironmentScope = "environment-scope"
	FlagTargetScope      = "target-scope"
	FlagStepScope        = "step-scope"
	FlagRoleScope        = "role-scope"
	FlagChannelScope     = "channel-scope"
	FlagTagScope         = "tag-scope"
	FlagProcessScope     = "process-scope"

	FlagType        = "type"
	FlagDescription = "description"

	TypeText      = "text"
	TypeSensitive = "sensitive"
)

type SetFlags struct {
	Project     *flag.Flag[string]
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Value       *flag.Flag[string]

	Type               *flag.Flag[string]
	EnvironmentsScopes *flag.Flag[[]string]
	ChannelScopes      *flag.Flag[[]string]
	TargetScopes       *flag.Flag[[]string]
	StepScopes         *flag.Flag[[]string]
	RoleScopes         *flag.Flag[[]string]
	TagScopes          *flag.Flag[[]string]
	ProcessScopes      *flag.Flag[[]string]
}

type SetOptions struct {
	*SetFlags
	*cmd.Dependencies
	shared.GetProjectCallback
}

func NewSetFlags() *SetFlags {
	return &SetFlags{
		Project:            flag.New[string](FlagProject, false),
		Name:               flag.New[string](FlagName, false),
		Value:              flag.New[string](FlagValue, false),
		Description:        flag.New[string](FlagDescription, false),
		Type:               flag.New[string](FlagType, false),
		EnvironmentsScopes: flag.New[[]string](FlagEnvironmentScope, false),
		ChannelScopes:      flag.New[[]string](FlagChannelScope, false),
		TargetScopes:       flag.New[[]string](FlagTargetScope, false),
		StepScopes:         flag.New[[]string](FlagStepScope, false),
		RoleScopes:         flag.New[[]string](FlagRoleScope, false),
		TagScopes:          flag.New[[]string](FlagTagScope, false),
		ProcessScopes:      flag.New[[]string](FlagProcessScope, false),
	}
}

func NewSetOptions(flags *SetFlags, dependencies *cmd.Dependencies) *SetOptions {
	return &SetOptions{
		SetFlags:     flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
	}
}

func NewSetCmd(f factory.Factory) *cobra.Command {
	setFlags := NewSetFlags()
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set project variables",
		Long:  "Set project variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable set
			$ %[1]s project variable set --name varname --value "abc"
			$ %[1]s project variable set --name varname --value "passwordABC" --type sensitive
			$ %[1]s project variable set --name varname --value "abc" --scope environment='test'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewSetOptions(setFlags, cmd.NewDependencies(f, c))
			if opts.Type.Value == TypeSensitive {
				opts.Value.Secure = true
			}

			return setRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&setFlags.Project.Value, setFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&setFlags.Name.Value, setFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVarP(&setFlags.Type.Value, setFlags.Type.Name, "t", TypeText, "The type of variable. Valid values are 'text', 'sensitive'. Default is 'text'")
	flags.StringVar(&setFlags.Value.Value, setFlags.Value.Name, "", "The value to set on the variable")
	flags.StringSliceVar(&setFlags.EnvironmentsScopes.Value, setFlags.EnvironmentsScopes.Name, []string{}, "Assign environment scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.ChannelScopes.Value, setFlags.ChannelScopes.Name, []string{}, "Assign channel scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.TargetScopes.Value, setFlags.TargetScopes.Name, []string{}, "Assign deployment target scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.StepScopes.Value, setFlags.StepScopes.Name, []string{}, "Assign process step scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.RoleScopes.Value, setFlags.RoleScopes.Name, []string{}, "Assign role scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.TagScopes.Value, setFlags.TagScopes.Name, []string{}, "Assign tag scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&setFlags.ProcessScopes.Value, setFlags.ProcessScopes.Name, []string{}, "Assign process scopes to the variable. Valid scopes are 'deployment' or a runbook name. Multiple scopes can be supplied.")

	return cmd
}

func setRun(opts *SetOptions) error {
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

	scope, err := toScope(projectVariables, opts, project)
	if err != nil {
		return err
	}
	vars, err := opts.Client.Variables.GetByName(project.GetID(), opts.Name.Value, scope)
	if err != nil {
		return err
	}

	if util.Empty(vars) {
		newVariable := variables.NewVariable(opts.Name.Value)
		newVariable.IsSensitive = opts.Type.Value == TypeSensitive
		newVariable.Value = opts.Value.Value
		newVariable.Scope = *scope

		_, err := opts.Client.Variables.AddSingle(project.GetID(), newVariable)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(opts.Out, "Successfully added variable '%s' in project '%s'", opts.Name.Value, project.GetName())
	} else if len(vars) == 1 {
		vars[0].Value = opts.Value.Value
		_, err := opts.Client.Variables.UpdateSingle(project.GetID(), vars[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(opts.Out, "Successfully updated variable '%s' in project '%s'", opts.Name.Value, project.GetName())
	} else if len(vars) > 1 {
		return fmt.Errorf("found multiple matching variables, aborting")
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Value, opts.Description, opts.Type, opts.EnvironmentsScopes, opts.ChannelScopes, opts.StepScopes, opts.TargetScopes, opts.TagScopes, opts.RoleScopes, opts.ProcessScopes)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func toScope(projectVariables variables.VariableSet, opts *SetOptions, project *projects.Project) (*variables.VariableScope, error) {
	scope := &variables.VariableScope{}
	var err error
	scope.Environments, err = buildSingleScope(opts.EnvironmentsScopes.Value, projectVariables.ScopeValues.Environments)
	if err != nil {
		return nil, err
	}

	scope.Roles, err = buildSingleScope(opts.RoleScopes.Value, projectVariables.ScopeValues.Roles)
	if err != nil {
		return nil, err
	}

	scope.Machines, err = buildSingleScope(opts.TargetScopes.Value, projectVariables.ScopeValues.Machines)
	if err != nil {
		return nil, err
	}

	scope.TenantTags, err = buildSingleScope(opts.TagScopes.Value, projectVariables.ScopeValues.TenantTags)
	if err != nil {
		return nil, err
	}

	scope.Actions, err = buildSingleScope(opts.StepScopes.Value, projectVariables.ScopeValues.Actions)
	if err != nil {
		return nil, err
	}

	scope.Channels, err = buildSingleScope(opts.ChannelScopes.Value, projectVariables.ScopeValues.Channels)
	if err != nil {
		return nil, err
	}

	processScopeReference := convertProcessScopesToReference(projectVariables.ScopeValues.Processes)
	processScopeReference = append(processScopeReference, &resources.ReferenceDataItem{ID: project.GetID(), Name: "deployment"})
	scope.ProcessOwners, err = buildSingleScope(opts.ProcessScopes.Value, processScopeReference)
	if err != nil {
		return nil, err
	}

	return scope, nil
}

func convertProcessScopesToReference(processes []*resources.ProcessReferenceDataItem) []*resources.ReferenceDataItem {
	refs := []*resources.ReferenceDataItem{}
	for _, p := range processes {
		refs = append(refs, &resources.ReferenceDataItem{
			ID:   p.ID,
			Name: p.Name,
		})
	}

	return refs
}

func buildSingleScope(inputScopes []string, references []*resources.ReferenceDataItem) ([]string, error) {
	scopes := []string{}
	for _, e := range inputScopes {
		ref, err := findReference(e, references)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, ref)
	}

	return scopes, nil
}

func findReference(value string, items []*resources.ReferenceDataItem) (string, error) {
	for _, i := range items {
		if strings.EqualFold(value, i.ID) || strings.EqualFold(value, i.Name) {
			return i.ID, nil
		}
	}

	return "", fmt.Errorf("cannot find scope value '%s'", value)
}

func PromptMissing(opts *SetOptions) error {
	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    fmt.Sprintf("A name for this variable."),
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	question.AskDescription(opts.Ask, "", "Variable", &opts.Description.Value)

	return nil
}
