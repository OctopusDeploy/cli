package list

import (
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/featuretoggle"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
)

const (
	LibraryVariableSetType = "Library"
	ProjectType            = "Project"

	FlagTenant = "tenant"
)

type ListFlags struct {
	Tenant *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Tenant: flag.New[string](FlagTenant, false),
	}
}

type VariableValue struct {
	Type            string
	OwnerName       string
	Name            string
	Value           string
	Label           string
	IsSensitive     bool
	IsDefaultValue  bool
	ScopeName       string
	HasMissingValue bool
}

type VariableValueAsJson struct {
	Type           string
	OwnerName      string
	Name           string
	Value          string
	Label          string
	IsSensitive    bool
	IsDefaultValue bool
}

type VariableValueProjectAsJson struct {
	*VariableValueAsJson
	HasMissingValue bool
	Environment     string
}

func NewVariableValueAsJson(v *VariableValue) *VariableValueAsJson {
	return &VariableValueAsJson{
		Type:           v.Type,
		OwnerName:      v.OwnerName,
		Name:           v.Name,
		Value:          v.Value,
		IsSensitive:    v.IsSensitive,
		IsDefaultValue: v.IsDefaultValue,
	}
}

func NewVariableValueProjectAsJson(v *VariableValue) *VariableValueProjectAsJson {
	return &VariableValueProjectAsJson{
		VariableValueAsJson: NewVariableValueAsJson(v),
		Environment:         v.ScopeName,
		HasMissingValue:     v.HasMissingValue,
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	cmd := &cobra.Command{
		Use:   "list [<name> | <id>]",
		Short: "List tenant variables",
		Long:  "List tenant variables in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s tenant variables list "Bobs Wood Shop"
			%[1]s tenant variables list --tenant "Bobs Wood Shop"
			%[1]s tenant variables ls Tenant-123
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		Args:    usage.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return listRun(c, f, resolveTenantIdentifier(listFlags.Tenant.Value, args))
		},
	}

	// `tenant variables update` identifies its tenant with --tenant/-t; accept the
	// same flag here so the two subcommands can be driven the same way. The
	// positional argument is kept for backwards compatibility.
	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Tenant.Value, listFlags.Tenant.Name, "t", "", "The tenant")

	return cmd
}

// resolveTenantIdentifier prefers --tenant and falls back to the positional
// argument, matching how the other commands accepting both forms behave.
//
// An empty result means no tenant was named; the caller prompts for one.
func resolveTenantIdentifier(tenant string, args []string) string {
	if tenant != "" {
		return tenant
	}

	if len(args) > 0 {
		return args[0]
	}

	return ""
}

// promptMissing prompts for a tenant when none was named on the command line,
// matching what `tenant variables update` does. The getter is a parameter so
// the prompt can be driven from tests, as connect and update do.
func promptMissing(ask question.Asker, getAllTenants shared.GetAllTenantsCallback) (*tenants.Tenant, error) {
	return selectors.Select(
		ask,
		"You have not specified a Tenant. Please select one:",
		getAllTenants,
		func(tenant *tenants.Tenant) string { return tenant.Name })
}

func listRun(cmd *cobra.Command, f factory.Factory, id string) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	var tenant *tenants.Tenant
	if id == "" {
		// with prompting disabled there is nothing to select from
		if !f.IsPromptEnabled() {
			return fmt.Errorf("must supply tenant identifier")
		}

		tenant, err = promptMissing(f.Ask, func() ([]*tenants.Tenant, error) { return shared.GetAllTenants(client) })
	} else {
		tenant, err = client.Tenants.GetByIdentifier(id)
	}
	if err != nil {
		return err
	}

	toggleValue, err := featuretoggle.IsToggleEnabled(client, "CommonVariableScopingFeatureToggle")

	if toggleValue {
		projectVariablesQuery := variables.GetTenantProjectVariablesQuery{
			TenantID:                tenant.ID,
			SpaceID:                 tenant.SpaceID,
			IncludeMissingVariables: true,
		}

		projectVariables, err := tenants.GetProjectVariables(client, projectVariablesQuery)
		if err != nil {
			return err
		}

		commonVariablesQuery := variables.GetTenantCommonVariablesQuery{
			TenantID:                tenant.ID,
			SpaceID:                 tenant.SpaceID,
			IncludeMissingVariables: true,
		}

		commonVariables, err := tenants.GetCommonVariables(client, commonVariablesQuery)
		if err != nil {
			return err
		}

		environmentMap, err := getEnvironmentMap(client)
		if err != nil {
			return err
		}

		var allVariableValues []*VariableValue

		for _, element := range commonVariables.MissingVariables {
			variableValue := unwrapCommonVariables(element, environmentMap)
			allVariableValues = append(allVariableValues, variableValue...)
		}

		for _, element := range commonVariables.Variables {
			variableValue := unwrapCommonVariables(element, environmentMap)
			allVariableValues = append(allVariableValues, variableValue...)
		}

		for _, element := range projectVariables.MissingVariables {
			variableValue := unwrapProjectVariables(element, environmentMap)
			allVariableValues = append(allVariableValues, variableValue...)
		}

		for _, element := range projectVariables.Variables {
			variableValue := unwrapProjectVariables(element, environmentMap)
			allVariableValues = append(allVariableValues, variableValue...)
		}

		sortVariableOutput(allVariableValues)

		return output.PrintArray(allVariableValues, cmd, output.Mappers[*VariableValue]{
			Json: func(item *VariableValue) any {
				if item.Type == LibraryVariableSetType {
					return NewVariableValueAsJson(item)
				} else {
					return NewVariableValueProjectAsJson(item)
				}
			},
			Table: output.TableDefinition[*VariableValue]{
				Header: []string{"NAME", "LABEL", "TYPE", "OWNER", "ENVIRONMENT", "VALUE", "SENSITIVE", "DEFAULT VALUE"},
				Row: func(item *VariableValue) []string {
					value := item.Value
					if item.HasMissingValue {
						value = output.Red("<missing>")
					}

					return []string{output.Bold(item.Name), item.Label, item.Type, item.OwnerName, item.ScopeName, value, fmt.Sprint(item.IsSensitive), fmt.Sprint(item.IsDefaultValue)}
				},
			},
			Basic: func(item *VariableValue) string {
				return item.Name
			},
		})

		return nil
	} else {
		vars, err := client.Tenants.GetVariables(tenant)
		if err != nil {
			return err
		}

		missingVariablesResponse, err := client.Tenants.GetMissingVariables(variables.MissingVariablesQuery{TenantID: vars.TenantID})
		var missingVariables []variables.MissingVariable
		if len(*missingVariablesResponse) > 0 {
			missingVariables = (*missingVariablesResponse)[0].MissingVariables
		}
		var allVariableValues []*VariableValue
		for _, element := range vars.LibraryVariables {

			variableValues := unwrapCommonVariablesV1(element, missingVariables)
			for _, v := range variableValues {
				allVariableValues = append(allVariableValues, v)
			}
		}

		if err != nil {
			return err
		}
		environmentMap, err := getEnvironmentMap(client)
		if err != nil {
			return err
		}
		for _, element := range vars.ProjectVariables {
			variableValues := unwrapProjectVariablesV1(element, environmentMap, missingVariables)
			for _, v := range variableValues {
				allVariableValues = append(allVariableValues, v)
			}
		}

		return output.PrintArray(allVariableValues, cmd, output.Mappers[*VariableValue]{
			Json: func(item *VariableValue) any {
				if item.Type == LibraryVariableSetType {
					return NewVariableValueAsJson(item)
				} else {
					return NewVariableValueProjectAsJson(item)
				}
			},
			Table: output.TableDefinition[*VariableValue]{
				Header: []string{"NAME", "LABEL", "TYPE", "OWNER", "ENVIRONMENT", "VALUE", "SENSITIVE", "DEFAULT VALUE"},
				Row: func(item *VariableValue) []string {
					value := item.Value
					if item.HasMissingValue {
						value = output.Red("<missing>")
					}

					return []string{output.Bold(item.Name), item.Label, item.Type, item.OwnerName, item.ScopeName, value, fmt.Sprint(item.IsSensitive), fmt.Sprint(item.IsDefaultValue)}
				},
			},
			Basic: func(item *VariableValue) string {
				return item.Name
			},
		})

		return nil
	}
}

func sortVariableOutput(allVariableValues []*VariableValue) {
	sort.SliceStable(allVariableValues, func(i, j int) bool {
		if allVariableValues[i].Type != allVariableValues[j].Type {
			return allVariableValues[i].Type < allVariableValues[j].Type
		}
		if allVariableValues[i].OwnerName != allVariableValues[j].OwnerName {
			return allVariableValues[i].OwnerName < allVariableValues[j].OwnerName
		}
		if allVariableValues[i].Name != allVariableValues[j].Name {
			return allVariableValues[i].Name < allVariableValues[j].Name
		}
		if allVariableValues[i].Value == "" && allVariableValues[j].Value != "" {
			return true
		}
		if allVariableValues[i].Value != "" && allVariableValues[j].Value == "" {
			return false
		}
		return allVariableValues[i].Value < allVariableValues[j].Value
	})
}

func unwrapCommonVariablesV1(variables variables.LibraryVariable, missingVariables []variables.MissingVariable) []*VariableValue {
	var results []*VariableValue = nil
	for _, template := range variables.Templates {
		value, isDefault := getVariableValue(template, variables.Variables)
		hasMissingValue := hasMissingCommonValue(missingVariables, variables.LibraryVariableSetID, template.ID)
		actualValue := getDisplayValue(value)

		results = append(results, &VariableValue{
			Type:            LibraryVariableSetType,
			OwnerName:       variables.LibraryVariableSetName,
			Name:            template.Name,
			Value:           actualValue,
			Label:           template.Label,
			IsSensitive:     value.IsSensitive,
			IsDefaultValue:  isDefault,
			HasMissingValue: hasMissingValue,
			ScopeName:       "",
		})
	}

	return results
}

func unwrapProjectVariablesV1(variables variables.ProjectVariable, environmentMap map[string]string, missingVariables []variables.MissingVariable) []*VariableValue {
	var results []*VariableValue = nil
	for _, template := range variables.Templates {
		for environmentId, environmentVariables := range variables.Variables {
			value, isDefault := getVariableValue(template, environmentVariables)
			hasMissingValue := hasMissingProjectValue(missingVariables, variables.ProjectID, environmentId, template.ID)
			displayValue := getDisplayValue(value)

			results = append(results, &VariableValue{
				Type:            ProjectType,
				OwnerName:       variables.ProjectName,
				Name:            template.Name,
				Value:           displayValue,
				Label:           template.Label,
				IsSensitive:     value.IsSensitive,
				IsDefaultValue:  isDefault,
				ScopeName:       environmentMap[environmentId],
				HasMissingValue: hasMissingValue,
			})
		}
	}

	return results
}

func unwrapCommonVariables(variable variables.TenantCommonVariable, environmentMap map[string]string) []*VariableValue {
	var results []*VariableValue = nil
	var isDefault = variable.ID == ""
	var displayValue string

	if isDefault {
		displayValue = getDisplayValue(variable.Template.DefaultValue)
	} else {
		displayValue = getDisplayValue(&variable.Value)
	}

	if len(variable.Scope.EnvironmentIds) == 0 {
		results = append(results, &VariableValue{
			Type:            LibraryVariableSetType,
			OwnerName:       variable.LibraryVariableSetName,
			Name:            variable.Template.Name,
			Value:           displayValue,
			IsSensitive:     variable.Value.IsSensitive,
			IsDefaultValue:  isDefault, // Variables without an ID are sourced from the MissingVariables list. For consistency with V1, IsDefault applies to both default and missing variables
			ScopeName:       "Unscoped",
			HasMissingValue: isDefault && displayValue == "",
		})

		return results
	}

	for _, environmentId := range variable.Scope.EnvironmentIds {

		results = append(results, &VariableValue{
			Type:            LibraryVariableSetType,
			OwnerName:       variable.LibraryVariableSetName,
			Name:            variable.Template.Name,
			Value:           displayValue,
			IsSensitive:     variable.Value.IsSensitive,
			IsDefaultValue:  isDefault, // Variables without an ID are sourced from the MissingVariables list. For consistency with V1, IsDefault applies to both default and missing variables
			ScopeName:       environmentMap[environmentId],
			HasMissingValue: isDefault && displayValue == "",
		})
	}

	return results
}

func unwrapProjectVariables(variable variables.TenantProjectVariable, environmentMap map[string]string) []*VariableValue {
	var results []*VariableValue = nil
	var isDefault = variable.ID == ""
	var displayValue string

	if isDefault {
		displayValue = getDisplayValue(variable.Template.DefaultValue)
	} else {
		displayValue = getDisplayValue(&variable.Value)
	}

	if len(variable.Scope.EnvironmentIds) == 0 {
		results = append(results, &VariableValue{
			Type:            ProjectType,
			OwnerName:       variable.ProjectName,
			Name:            variable.Template.Name,
			Value:           displayValue,
			IsSensitive:     variable.Value.IsSensitive,
			IsDefaultValue:  isDefault, // Variables without an ID are sourced from the MissingVariables list. For consistency with V1, IsDefault applies to both default and missing variables
			ScopeName:       "Unscoped",
			HasMissingValue: isDefault && displayValue == "",
		})

		return results
	}

	for _, environmentId := range variable.Scope.EnvironmentIds {

		results = append(results, &VariableValue{
			Type:            ProjectType,
			OwnerName:       variable.ProjectName,
			Name:            variable.Template.Name,
			Value:           displayValue,
			IsSensitive:     variable.Value.IsSensitive,
			IsDefaultValue:  isDefault, // Variables without an ID are sourced from the MissingVariables list. For consistency with V1, IsDefault applies to both default and missing variables
			ScopeName:       environmentMap[environmentId],
			HasMissingValue: isDefault && displayValue == "",
		})
	}

	return results
}

func hasMissingProjectValue(missingVariables []variables.MissingVariable, projectID string, environmentID string, templateID string) bool {
	for _, v := range missingVariables {
		if v.ProjectID == projectID && v.EnvironmentID == environmentID && v.VariableTemplateID == templateID {
			return true
		}
	}

	return false
}

func hasMissingCommonValue(missingVariables []variables.MissingVariable, libraryVariableSetId string, templateId string) bool {
	for _, v := range missingVariables {
		if v.LibraryVariableSetID == v.LibraryVariableSetID && v.VariableTemplateID == templateId {
			return true
		}
	}
	return false
}

func getVariableValue(template *actiontemplates.ActionTemplateParameter, values map[string]core.PropertyValue) (*core.PropertyValue, bool) {
	if value, ok := values[template.ID]; ok {
		return &value, false
	}

	return template.DefaultValue, true
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

func getDisplayValue(value *core.PropertyValue) string {
	var actualValue string
	if value.IsSensitive {
		if value.SensitiveValue.HasValue {
			actualValue = "***"
		} else {
			actualValue = ""
		}
	} else {
		actualValue = value.Value
	}
	return actualValue
}
