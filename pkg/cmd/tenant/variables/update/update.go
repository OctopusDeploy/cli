package update

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagTenant             = "tenant"
	FlagProject            = "project"
	FlagEnvironment        = "environment"
	FlagLibraryVariableSet = "library-variable-set"
	FlagName               = "name"
	FlagValue              = "value"

	VariableOwnerTypeCommon  = "common"
	VariableOwnerTypeProject = "project"
)

type UpdateFlags struct {
	Tenant             *flag.Flag[string]
	Project            *flag.Flag[string]
	Environment        *flag.Flag[string]
	LibraryVariableSet *flag.Flag[string]
	Name               *flag.Flag[string]
	Value              *flag.Flag[string]
}

type UpdateOptions struct {
	*UpdateFlags
	VariableId string
	*cmd.Dependencies
	shared.GetProjectCallback
	shared.GetAllProjectsCallback
	shared.GetTenantCallback
	shared.GetAllTenantsCallback
	shared.GetAllLibraryVariableSetsCallback
	*sharedVariable.VariableCallbacks
}

func NewUpdateFlags() *UpdateFlags {
	return &UpdateFlags{
		Tenant:             flag.New[string](FlagTenant, false),
		Project:            flag.New[string](FlagProject, false),
		Environment:        flag.New[string](FlagEnvironment, false),
		LibraryVariableSet: flag.New[string](FlagLibraryVariableSet, false),
		Name:               flag.New[string](FlagName, false),
		Value:              flag.New[string](FlagValue, false),
	}
}

func NewUpdateOptions(flags *UpdateFlags, dependencies *cmd.Dependencies) *UpdateOptions {
	return &UpdateOptions{
		UpdateFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		GetTenantCallback: func(identifier string) (*tenants.Tenant, error) {
			return shared.GetTenant(dependencies.Client, identifier)
		},
		GetAllTenantsCallback: func() ([]*tenants.Tenant, error) { return shared.GetAllTenants(dependencies.Client) },
		GetAllLibraryVariableSetsCallback: func() ([]*variables.LibraryVariableSet, error) {
			return shared.GetAllLibraryVariableSets(dependencies.Client)
		},
		VariableCallbacks: sharedVariable.NewVariableCallbacks(dependencies),
	}
}

func NewCmdUpdate(f factory.Factory) *cobra.Command {
	updateFlags := NewUpdateFlags()
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the value of a tenant variable",
		Long:  "Update the value of a tenant variable in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant variable update
			$ %[1]s tenant variable update --tenant "Bobs Fish Shack" --name "site-name" --value "Bobs Fish Shack" --project "Awesome Web Site" --environment "Test"
			$ %[1]s tenant variable update --tenant "Sallys Tackle Truck" --name dbPassword --value "12345" --library-variable-set "Shared Variables"
			`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewUpdateOptions(updateFlags, cmd.NewDependencies(f, c))

			return updateRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&updateFlags.Tenant.Value, updateFlags.Tenant.Name, "t", "", "The tenant")
	flags.StringVarP(&updateFlags.Project.Value, updateFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&updateFlags.Environment.Value, updateFlags.Environment.Name, "e", "", "The environment")
	flags.StringVarP(&updateFlags.LibraryVariableSet.Value, updateFlags.LibraryVariableSet.Name, "l", "", "The library variable set")
	flags.StringVarP(&updateFlags.Name.Value, updateFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVar(&updateFlags.Value.Value, updateFlags.Value.Name, "", "The value to set on the variable")

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	tenant, err := opts.Client.Tenants.GetByIdentifier(opts.Tenant.Value)
	if err != nil {
		return err
	}

	vars, err := opts.Client.Tenants.GetVariables(tenant)
	if err != nil {
		return err
	}

	if opts.LibraryVariableSet.Value != "" {
		err = updateCommonVariableValue(opts, vars)
		if err != nil {
			return err
		}
	} else {
		environmentMap, err := getEnvironmentMap(opts.Client)
		if err != nil {
			return err
		}
		err = updateProjectVariableValue(opts, vars, environmentMap)
		if err != nil {
			return err
		}
	}

	_, err = opts.Client.Tenants.UpdateVariables(tenant, vars)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully updated variable '%s' in for tenant '%s'\n", opts.Name.Value, tenant.Name)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Tenant, opts.Name, opts.Value, opts.Project, opts.LibraryVariableSet, opts.Environment)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *UpdateOptions) error {
	var tenant *tenants.Tenant
	var err error
	if opts.Tenant.Value == "" {
		tenant, err = selectors.Select(opts.Ask, "You have not specified a source Tenant to clone from. Please select one:", opts.GetAllTenantsCallback, func(tenant *tenants.Tenant) string {
			return tenant.Name
		})
		if err != nil {
			return err
		}

		opts.Tenant.Value = tenant.Name
	} else {
		tenant, err = opts.Client.Tenants.GetByIdentifier(opts.Tenant.Value)
		if err != nil {
			return err
		}
	}

	var variableType = ""
	if opts.LibraryVariableSet.Value != "" {
		variableType = VariableOwnerTypeCommon
	} else if opts.Project.Value != "" {
		variableType = VariableOwnerTypeProject
	} else {
		selectedOption, err := selectors.SelectOptions(opts.Ask, "Which type of variable do you want to update?", getVariableTypeOptions)
		if err != nil {
			return err
		}
		variableType = selectedOption.Value
	}

	variables, err := opts.Client.Tenants.GetVariables(tenant)
	if err != nil {
		return err
	}

	switch variableType {
	case VariableOwnerTypeCommon:
		possibleVariables := findPossibleCommonVariables(opts, variables)

		if opts.LibraryVariableSet.Value == "" || opts.Name.Value == "" {
			selectedVariable, err := selectors.Select(opts.Ask, "You have not specified a variable", func() ([]*PossibleVariable, error) { return possibleVariables, nil }, func(variable *PossibleVariable) string {
				return fmt.Sprintf("%s / %s", variable.Owner, variable.VariableName)
			})
			if err != nil {
				return err
			}
			opts.LibraryVariableSet.Value = selectedVariable.Owner
			opts.Name.Value = selectedVariable.VariableName
			opts.VariableId = selectedVariable.ID
		}
	case VariableOwnerTypeProject:
		possibleVariables := findPossibleProjectVariables(opts, variables)
		if opts.Project.Value == "" || opts.Name.Value == "" {
			selectedVariable, err := selectors.Select(opts.Ask, "You have not specified a variable", func() ([]*PossibleVariable, error) { return possibleVariables, nil }, func(variable *PossibleVariable) string {
				return fmt.Sprintf("%s / %s", variable.Owner, variable.VariableName)
			})
			if err != nil {
				return err
			}
			opts.Project.Value = selectedVariable.Owner
			opts.Name.Value = selectedVariable.VariableName
			opts.VariableId = selectedVariable.ID
		}

		if opts.Environment.Value == "" {
			var environmentSelections []*selectors.SelectOption[string]
			environmentMap, err := getEnvironmentMap(opts.Client)
			if err != nil {
				return err
			}

			for _, p := range variables.ProjectVariables {
				if strings.EqualFold(p.ProjectName, opts.Project.Value) {
					for k, _ := range p.Variables {
						environmentSelections = append(environmentSelections, selectors.NewSelectOption[string](environmentMap[k], environmentMap[k]))
					}
				}
			}

			selectedEnvironment, err := selectors.SelectOptions(opts.Ask, "You have not specified an environment", func() []*selectors.SelectOption[string] { return environmentSelections })
			if err != nil {
				return err
			}
			opts.Environment.Value = selectedEnvironment.Value
		}
	}

	variableControlType, err := getVariableType(opts, variables)

	if opts.Value.Value == "" {
		variableType, err := mapVariableControlTypeToVariableType(variableControlType)
		if err != nil {
			return err
		}
		value, err := sharedVariable.PromptValue(opts.Ask, variableType, opts.VariableCallbacks)
		if err != nil {
			return err
		}
		opts.Value.Value = value
	}

	return nil
}

func mapVariableControlTypeToVariableType(controlType variables.ControlType) (sharedVariable.VariableType, error) {
	switch controlType {
	case variables.ControlTypeSingleLineText, variables.ControlTypeMultiLineText:
		return sharedVariable.VariableTypeString, nil
	case variables.ControlTypeSensitive:
		return sharedVariable.VariableTypeSensitive, nil
		//case variables.ControlTypeCheckbox
		//	return sharedVariable.
	}

	return "", fmt.Errorf("cannot map control type '%s' to variable type", controlType)
}

func getVariableType(opts *UpdateOptions, tenantVariables *variables.TenantVariables) (variables.ControlType, error) {
	if opts.LibraryVariableSet.Value != "" {
		for _, v := range tenantVariables.LibraryVariables {
			if strings.EqualFold(v.LibraryVariableSetName, opts.LibraryVariableSet.Value) {
				template, err := findTemplate(v.Templates, opts.Name.Value)
				if err != nil {
					return "", err
				}
				return variables.ControlType(template.DisplaySettings["Octopus.ControlType"]), nil
			}
		}
	} else if opts.Project.Value != "" {
		for _, v := range tenantVariables.ProjectVariables {
			if strings.EqualFold(v.ProjectName, opts.Project.Value) {
				template, err := findTemplate(v.Templates, opts.Name.Value)
				if err != nil {
					return "", err
				}
				return variables.ControlType(template.DisplaySettings["Octopus.ControlType"]), nil
			}
		}
	}

	return "", fmt.Errorf("cannot find variable")
}

func findTemplate(templates []*actiontemplates.ActionTemplateParameter, variableName string) (*actiontemplates.ActionTemplateParameter, error) {
	for _, t := range templates {
		if strings.EqualFold(t.Name, variableName) {
			return t, nil
		}
	}

	return nil, fmt.Errorf("cannot find variable called %s", variableName)
}

func findPossibleCommonVariables(opts *UpdateOptions, tenantVariables *variables.TenantVariables) []*PossibleVariable {
	var filteredVariables []*PossibleVariable
	for _, l := range tenantVariables.LibraryVariables {
		for _, t := range l.Templates {
			if (opts.Name.Value == "" && opts.LibraryVariableSet.Value == "") || (opts.Name.Value != "" && strings.EqualFold(opts.Name.Value, t.Name)) || (opts.LibraryVariableSet.Value != "" && strings.EqualFold(l.LibraryVariableSetName, opts.LibraryVariableSet.Value)) {
				filteredVariables = append(filteredVariables, &PossibleVariable{
					Owner:        l.LibraryVariableSetName,
					VariableName: t.Name,
				})
			}
		}
	}

	return filteredVariables
}

func findPossibleProjectVariables(opts *UpdateOptions, tenantVariables *variables.TenantVariables) []*PossibleVariable {
	var filteredVariables []*PossibleVariable
	for _, p := range tenantVariables.ProjectVariables {
		for _, t := range p.Templates {
			if (opts.Name.Value == "" && opts.Project.Value == "") || (opts.Name.Value != "" && strings.EqualFold(opts.Name.Value, t.Name)) || (opts.LibraryVariableSet.Value != "" && strings.EqualFold(p.ProjectName, opts.Project.Value)) {
				filteredVariables = append(filteredVariables, &PossibleVariable{
					ID:           t.ID,
					Owner:        p.ProjectName,
					VariableName: t.Name,
				})
			}
		}
	}

	return filteredVariables
}
func updateProjectVariableValue(opts *UpdateOptions, vars *variables.TenantVariables, environmentMap map[string]string) error {
	var environmentId string
	for id, environment := range environmentMap {
		if strings.EqualFold(environment, opts.Environment.Value) || strings.EqualFold(id, opts.Environment.Value) {
			environmentId = id
		}
	}
	for _, v := range vars.ProjectVariables {
		if strings.EqualFold(v.ProjectName, opts.Project.Value) {
			for _, t := range v.Templates {
				if strings.EqualFold(t.Name, opts.Name.Value) {
					v.Variables[environmentId][t.ID] = core.PropertyValue{
						Value: opts.Value.Value,
					}

					return nil
				}
			}
		}
	}

	return fmt.Errorf("unable to find requested variable")
}

func updateCommonVariableValue(opts *UpdateOptions, vars *variables.TenantVariables) error {
	for _, v := range vars.LibraryVariables {
		if strings.EqualFold(v.LibraryVariableSetName, opts.LibraryVariableSet.Value) {
			for _, t := range v.Templates {
				if strings.EqualFold(t.Name, opts.Name.Value) {
					v.Variables[t.ID] = core.PropertyValue{
						Value: opts.Value.Value,
					}

					return nil
				}
			}
		}
	}

	return fmt.Errorf("unable to find requested variable")
}

func getEnvironmentMap(client *client.Client) (map[string]string, error) {
	environmentMap := make(map[string]string)
	allEnvs, err := selectors.GetAllEnvironments(client)
	if err != nil {
		return nil, err
	}
	for _, e := range allEnvs {
		environmentMap[e.GetID()] = e.GetName()
	}
	return environmentMap, nil
}

func getVariableTypeOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Library/Common", Value: VariableOwnerTypeCommon},
		{Display: "Project", Value: VariableOwnerTypeProject},
	}
}

type PossibleVariable struct {
	ID           string
	Owner        string
	VariableName string
}
