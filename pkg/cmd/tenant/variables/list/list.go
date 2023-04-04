package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/actiontemplates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
)

const (
	LibraryVariableSetType = "Library"
	ProjectType            = "Project"
)

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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenant variables",
		Long:  "List tenant variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant variable list
			$ %[1]s tenant variable ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must supply tenant identifier")
			}
			return listRun(cmd, f, args[0])
		},
	}

	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory, id string) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	tenant, err := client.Tenants.GetByIdentifier(id)
	if err != nil {
		return err
	}

	vars, err := client.Tenants.GetVariables(tenant)
	if err != nil {
		return err
	}

	var allVariableValues []*VariableValue
	for _, element := range vars.LibraryVariables {
		variableValues := unwrapCommonVariables(element)
		for _, v := range variableValues {
			allVariableValues = append(allVariableValues, v)
		}
	}

	missingVariables, err := client.Tenants.GetMissingVariables(variables.MissingVariablesQuery{TenantID: vars.TenantID})
	if err != nil {
		return err
	}
	environmentMap, err := getEnvironmentMap(client)
	if err != nil {
		return err
	}
	for _, element := range vars.ProjectVariables {
		variableValues := unwrapProjectVariables(element, environmentMap, (*missingVariables)[0].MissingVariables)
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
				if item.Type == ProjectType && item.HasMissingValue {
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

func unwrapCommonVariables(variables variables.LibraryVariable) []*VariableValue {
	var results []*VariableValue = nil
	for _, template := range variables.Templates {
		value, isDefault := getVariableValue(template, variables.Variables)
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

		results = append(results, &VariableValue{
			Type:           LibraryVariableSetType,
			OwnerName:      variables.LibraryVariableSetName,
			Name:           template.Name,
			Value:          actualValue,
			Label:          template.Label,
			IsSensitive:    value.IsSensitive,
			IsDefaultValue: isDefault,
			ScopeName:      "",
		})
	}

	return results
}

func unwrapProjectVariables(variables variables.ProjectVariable, environmentMap map[string]string, missingVariables []variables.MissingVariable) []*VariableValue {
	var results []*VariableValue = nil
	for _, template := range variables.Templates {
		for environmentId, environmentVariables := range variables.Variables {
			value, isDefault := getVariableValue(template, environmentVariables)

			hasMissingValue := hasMissingValue(missingVariables, variables.ProjectID, environmentId, template.ID)

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

			results = append(results, &VariableValue{
				Type:            ProjectType,
				OwnerName:       variables.ProjectName,
				Name:            template.Name,
				Value:           actualValue,
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

func hasMissingValue(missingVariables []variables.MissingVariable, projectID string, environmentID string, templateID string) bool {
	for _, v := range missingVariables {
		if v.ProjectID == projectID && v.EnvironmentID == environmentID && v.VariableTemplateID == templateID {
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
