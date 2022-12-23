package shared

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
)

func PromptValue(ask question.Asker, value *string, variableType string) error {
	switch variableType {
	case "String":
		if err := ask(&survey.Input{
			Message: "Value",
		}, &value); err != nil {
			return err
		}
	case "Sensitive":
		if err := ask(&survey.Password{
			Message: "Value",
		}, &value); err != nil {
			return err
		}
		// TODO: implement other value selectors
	}

	return nil
}

func PromptScopes(asker question.Asker, projectVariables variables.VariableSet, flags *ScopeFlags, isPrompted bool) error {
	var err error
	if util.Empty(flags.EnvironmentsScopes.Value) {
		flags.EnvironmentsScopes.Value, err = PromptScope(asker, "Environment", projectVariables.ScopeValues.Environments)
		if err != nil {
			return err
		}
	}

	if util.Empty(flags.ProcessScopes.Value) {
		flags.ProcessScopes.Value, err = PromptScope(asker, "Process", ConvertProcessScopesToReference(projectVariables.ScopeValues.Processes))
		if err != nil {
			return err
		}
	}

	if !isPrompted {
		if util.Empty(flags.ChannelScopes.Value) {
			flags.ChannelScopes.Value, err = PromptScope(asker, "Channel", projectVariables.ScopeValues.Channels)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.TargetScopes.Value) {
			flags.TargetScopes.Value, err = PromptScope(asker, "Target", projectVariables.ScopeValues.Machines)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.RoleScopes.Value) {
			flags.RoleScopes.Value, err = PromptScope(asker, "Role", projectVariables.ScopeValues.Roles)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.TagScopes.Value) {
			flags.TagScopes.Value, err = PromptScope(asker, "Tag", projectVariables.ScopeValues.TenantTags)
			if err != nil {
				return err
			}
		}

		if util.Empty(flags.StepScopes.Value) {
			flags.StepScopes.Value, err = PromptScope(asker, "Step", projectVariables.ScopeValues.Actions)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func PromptScope(ask question.Asker, scopeDescription string, items []*resources.ReferenceDataItem) ([]string, error) {
	if util.Empty(items) {
		return nil, nil
	}
	var selectedItems []string
	err := ask(&survey.MultiSelect{
		Message: fmt.Sprintf("%s scope", scopeDescription),
		Options: util.SliceTransform(items, func(i *resources.ReferenceDataItem) string { return i.Name }),
	}, &selectedItems)

	if err != nil {
		return nil, err
	}

	return selectedItems, nil
}
