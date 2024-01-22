package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedProjectVariable "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagProject     = "project"
	FlagName        = "name"
	FlagValue       = "value"
	FlagType        = "type"
	FlagDescription = "description"
	FlagGitRef      = "git-ref"

	FlagPrompt              = "prompted"
	FlagPromptLabel         = "prompt-label"
	FlagPromptDescription   = "prompt-description"
	FlagPromptType          = "prompt-type"
	FlagPromptRequired      = "prompt-required"
	FlagPromptSelectOptions = "prompt-dropdown-option"

	TypeText          = "text"
	TypeSensitive     = "sensitive"
	TypeAwsAccount    = "awsaccount"
	TypeWorkerPool    = "workerpool"
	TypeAzureAccount  = "azureaccount"
	TypeCertificate   = "certificate"
	TypeGoogleAccount = "googleaccount"

	PromptTypeText      = "text"
	PromptTypeMultiText = "multiline-text"
	PromptTypeCheckbox  = "checkbox"
	PromptTypeDropdown  = "dropdown"
)

type CreateFlags struct {
	Project     *flag.Flag[string]
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Value       *flag.Flag[string]
	Type        *flag.Flag[string]
	GitRef      *flag.Flag[string]

	*sharedProjectVariable.ScopeFlags

	IsPrompted          *flag.Flag[bool]
	PromptLabel         *flag.Flag[string]
	PromptDescription   *flag.Flag[string]
	PromptType          *flag.Flag[string]
	PromptRequired      *flag.Flag[bool]
	PromptSelectOptions *flag.Flag[[]string]
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	shared.GetProjectCallback
	shared.GetAllProjectsCallback
	*sharedVariable.VariableCallbacks
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:             flag.New[string](FlagProject, false),
		Name:                flag.New[string](FlagName, false),
		Value:               flag.New[string](FlagValue, false),
		Description:         flag.New[string](FlagDescription, false),
		GitRef:              flag.New[string](FlagGitRef, false),
		Type:                flag.New[string](FlagType, false),
		ScopeFlags:          sharedProjectVariable.NewScopeFlags(),
		IsPrompted:          flag.New[bool](FlagPrompt, false),
		PromptLabel:         flag.New[string](FlagPromptLabel, false),
		PromptDescription:   flag.New[string](FlagPromptDescription, false),
		PromptType:          flag.New[string](FlagPromptType, false),
		PromptRequired:      flag.New[bool](FlagPromptRequired, false),
		PromptSelectOptions: flag.New[[]string](FlagPromptSelectOptions, false),
	}
}

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		VariableCallbacks:      sharedVariable.NewVariableCallbacks(dependencies),
	}
}

func NewCreateCmd(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a variable for a project",
		Long:    "Create a variable for a project in Octopus Deploy",
		Aliases: []string{"add"},
		Example: heredoc.Docf(`
			$ %[1]s project variable create
			$ %[1]s project variable create --project "Deploy Website" --name varname --value "abc"
			$ %[1]s project variable create --name varname --value "passwordABC" --type sensitive
			$ %[1]s project variable create --name varname --value "abc" --scope environment='test'
			$ %[1]s project variable create --name varname --value "abc" --scope environment='test' --git-ref refs/heads/main
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))
			if opts.Type.Value == TypeSensitive {
				opts.Value.Secure = true
			}

			return CreateRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVarP(&createFlags.Type.Value, createFlags.Type.Name, "t", "", fmt.Sprintf("The type of variable. Valid values are %s. Default is %s", strings.Join([]string{TypeText, TypeSensitive, TypeWorkerPool, TypeAwsAccount, TypeAzureAccount, TypeGoogleAccount, TypeCertificate}, ", "), TypeText))
	flags.StringVar(&createFlags.Value.Value, createFlags.Value.Name, "", "The value to set on the variable")
	flags.StringVarP(&createFlags.GitRef.Value, createFlags.GitRef.Name, "", "", "The GitRef for the Config-As-Code branch")

	sharedProjectVariable.RegisterScopeFlags(cmd, createFlags.ScopeFlags)
	flags.BoolVar(&createFlags.IsPrompted.Value, createFlags.IsPrompted.Name, false, "Make a prompted variable")
	flags.StringVar(&createFlags.PromptLabel.Value, createFlags.PromptLabel.Name, "", "The label for the prompted variable")
	flags.StringVar(&createFlags.PromptDescription.Value, createFlags.PromptDescription.Name, "", "Description for the prompted variable")
	flags.StringVar(&createFlags.PromptType.Value, createFlags.PromptType.Name, "", fmt.Sprintf("The input type for the prompted variable. Valid values are '%s', '%s', '%s' and '%s'", PromptTypeText, PromptTypeMultiText, PromptTypeCheckbox, PromptTypeDropdown))
	flags.BoolVar(&createFlags.PromptRequired.Value, createFlags.PromptRequired.Name, false, "Prompt will require a value for deployment")
	flags.StringSliceVar(&createFlags.PromptSelectOptions.Value, createFlags.PromptSelectOptions.Name, []string{}, "Options for a dropdown prompt. May be specified multiple times. Must be in format 'value|description'")
	return cmd
}

func CreateRun(opts *CreateOptions) error {
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

	var projectVariables *variables.VariableSet
	if project.IsVersionControlled && opts.GitRef.Value != "" {
		projectVariables, err = opts.GetProjectVariablesByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value)
	} else {
		projectVariables, err = opts.GetProjectVariables(project.GetID())
	}
	if err != nil {
		return err
	}

	scope, err := sharedProjectVariable.ToVariableScope(projectVariables, opts.ScopeFlags, project)
	if err != nil {
		return err
	}

	newVariable := variables.NewVariable(opts.Name.Value)
	varType, err := mapVariableType(opts.Type.Value)
	if err != nil {
		return err
	}

	newVariable.Type = varType
	newVariable.Value = opts.Value.Value
	newVariable.Scope = *scope

	if opts.IsPrompted.Value {
		promptControlType, err := mapControlType(opts.PromptType.Value)
		if err != nil {
			return err
		}
		newVariable.Prompt = &variables.VariablePromptOptions{
			Description: opts.PromptDescription.Value,
			Label:       opts.PromptLabel.Value,
			IsRequired:  opts.PromptRequired.Value,
		}

		selectOptions := parseSelectOptions(opts, promptControlType)
		newVariable.Prompt.DisplaySettings = resources.NewDisplaySettings(promptControlType, selectOptions)
	}

	if opts.GitRef.Value != "" {
		_, err = opts.Client.ProjectVariables.AddSingleByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value, newVariable)
	} else {
		_, err = opts.Client.Variables.AddSingle(project.GetID(), newVariable)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created variable '%s' in project '%s'\n", opts.Name.Value, project.GetName())

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.Name, opts.Value, opts.Description, opts.Type, opts.EnvironmentsScopes, opts.ChannelScopes, opts.StepScopes, opts.TargetScopes, opts.TagScopes, opts.RoleScopes, opts.ProcessScopes, opts.IsPrompted, opts.PromptType, opts.PromptLabel, opts.PromptDescription, opts.PromptSelectOptions, opts.PromptRequired, opts.GitRef)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {
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

	err = PromptVersionControl(opts, project)
	if err != nil {
		return err
	}

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

	if opts.Type.Value == "" {
		selectedType, err := selectors.SelectOptions(opts.Ask, "Select the type of the variable", getVariableTypeOptions)
		if err != nil {
			return err
		}
		opts.Type.Value = selectedType.Value
	}

	if !opts.IsPrompted.Value {
		opts.Ask(&survey.Confirm{
			Message: "Is this a prompted variable?",
			Default: false,
		}, &opts.IsPrompted.Value)
	}

	if opts.IsPrompted.Value {
		if opts.PromptLabel.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Prompt Label",
			}, &opts.PromptLabel.Value); err != nil {
				return err
			}
		}

		question.AskDescription(opts.Ask, "Prompt ", "Prompted Variable", &opts.PromptDescription.Value)

		if opts.PromptType.Value == "" {
			selectedPromptType, err := selectors.SelectOptions(opts.Ask, "Select the control type of the prompted variable", getControlTypeOptions)
			if err != nil {
				return err
			}
			opts.PromptType.Value = selectedPromptType.Value
		}

		if opts.PromptType.Value == PromptTypeDropdown && util.Empty(opts.PromptSelectOptions.Value) {
			for {
				var value string

				if err := opts.Ask(&survey.Input{
					Message: "Enter a selection option value (enter blank to end)",
				}, &value, survey.WithValidator(survey.MaxLength(200))); err != nil {
					return err
				}

				if strings.TrimSpace(value) == "" {
					break
				}

				var description string
				if err := opts.Ask(&survey.Input{
					Message: "Enter a selection option description",
				}, &description, survey.WithValidator(survey.ComposeValidators(survey.Required))); err != nil {
					return err
				}

				opts.PromptSelectOptions.Value = append(opts.PromptSelectOptions.Value, fmt.Sprintf("%s|%s", value, description))
			}
		}

		if !opts.PromptRequired.Value {
			if err := opts.Ask(&survey.Confirm{
				Message: "Is this the prompted variable required to have a value supplied?",
				Default: false,
			}, &opts.PromptRequired.Value); err != nil {
				return err
			}
		}

	}

	if opts.Value.Value == "" {
		variableType, err := mapVariableType(opts.Type.Value)
		if err != nil {
			return err
		}
		opts.Value.Value, err = sharedVariable.PromptValue(opts.Ask, sharedVariable.VariableType(variableType), opts.VariableCallbacks, nil)
		if err != nil {
			return err
		}
	}

	var projectVariables *variables.VariableSet
	if opts.GitRef.Value != "" {
		projectVariables, err = opts.GetProjectVariablesByGitRef(opts.Space.GetID(), project.GetID(), opts.GitRef.Value)
	} else {
		projectVariables, err = opts.GetProjectVariables(project.GetID())
	}

	if err != nil {
		return err
	}

	scope, err := sharedProjectVariable.ToVariableScope(projectVariables, opts.ScopeFlags, project)
	if err != nil {
		return err
	}

	if scope.IsEmpty() {
		err = sharedProjectVariable.PromptScopes(opts.Ask, projectVariables, opts.ScopeFlags, opts.IsPrompted.Value)
		if err != nil {
			return err
		}
	}

	return nil
}

func PromptVersionControl(opts *CreateOptions, project *projects.Project) error {
	if !project.IsVersionControlled {
		return nil
	}

	if opts.GitRef.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "GitRef",
			Help:    fmt.Sprintf("The GitRef where the variable is stored"),
		}, &opts.GitRef.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	return nil
}

func getVariableTypeOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Text", Value: TypeText},
		{Display: "Sensitive", Value: TypeSensitive},
		{Display: "Certificate", Value: TypeCertificate},
		{Display: "Worker Pool", Value: TypeWorkerPool},
		{Display: "Azure Account", Value: TypeAzureAccount},
		{Display: "Aws Account", Value: TypeAwsAccount},
		{Display: "Google Account", Value: TypeGoogleAccount},
	}
}

func getControlTypeOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Single line text", Value: PromptTypeText},
		{Display: "Multi line text", Value: PromptTypeMultiText},
		{Display: "Checkbox", Value: PromptTypeCheckbox},
		{Display: "Drop down", Value: PromptTypeDropdown},
	}
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string { return p.GetName() })
}

func parseSelectOptions(opts *CreateOptions, controlType resources.ControlType) []*resources.SelectOption {
	options := []*resources.SelectOption{}
	if controlType != resources.ControlTypeSelect {
		return options
	}

	for _, selectOption := range opts.PromptSelectOptions.Value {
		o := strings.Split(selectOption, "|")
		options = append(options, &resources.SelectOption{
			Value:       o[0],
			DisplayName: o[1],
		})
	}

	return options
}

func mapVariableType(varType string) (string, error) {
	if varType == "" {
		varType = TypeText
	}

	switch varType {
	case TypeText:
		return "String", nil
	case TypeSensitive:
		return "Sensitive", nil
	case TypeAwsAccount:
		return "AmazonWebServicesAccount", nil
	case TypeWorkerPool:
		return "WorkerPool", nil
	case TypeAzureAccount:
		return "AzureAccount", nil
	case TypeCertificate:
		return "Certificate", nil
	case TypeGoogleAccount:
		return "GoogleCloudAccount", nil
	default:
		return "", fmt.Errorf("unknown variable type '%s', valid values are '%s','%s','%s', '%s', '%s', '%s', '%s'", varType, TypeText, TypeSensitive, TypeAzureAccount, TypeAwsAccount, TypeGoogleAccount, TypeWorkerPool, TypeCertificate)
	}
}

func mapControlType(promptType string) (resources.ControlType, error) {
	switch promptType {
	case PromptTypeText:
		return resources.ControlTypeSingleLineText, nil
	case PromptTypeMultiText:
		return resources.ControlTypeMultiLineText, nil
	case PromptTypeCheckbox:
		return resources.ControlTypeCheckbox, nil
	case PromptTypeDropdown:
		return resources.ControlTypeSelect, nil
	default:
		return "", fmt.Errorf("unknown prompt type '%s', valid values are '%s','%s','%s', '%s'", promptType, PromptTypeText, PromptTypeMultiText, PromptTypeCheckbox, PromptTypeDropdown)
	}
}
