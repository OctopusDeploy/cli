package shared

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
)

func PromptScopes(asker question.Asker, projectVariables *variables.VariableSet, flags *ScopeFlags, isPrompted bool) error {
	var err error
	if util.Empty(flags.EnvironmentsScopes.Value) {
		flags.EnvironmentsScopes.Value, err = PromptScope(asker, "Environment", projectVariables.ScopeValues.Environments, nil)
		if err != nil {
			return err
		}
	}

	flags.ProcessScopes.Value, err = PromptScope(asker, "Process", ConvertProcessScopesToReference(projectVariables.ScopeValues.Processes), nil)
	if err != nil {
		return err
	}

	if !isPrompted {
		flags.ChannelScopes.Value, err = PromptScope(asker, "Channel", projectVariables.ScopeValues.Channels, nil)
		if err != nil {
			return err
		}

		flags.TargetScopes.Value, err = PromptScope(asker, "Target", projectVariables.ScopeValues.Machines, nil)
		if err != nil {
			return err
		}

		flags.RoleScopes.Value, err = PromptScope(asker, "Role", projectVariables.ScopeValues.Roles, nil)
		if err != nil {
			return err
		}

		flags.TagScopes.Value, err = PromptScope(asker, "Tag", projectVariables.ScopeValues.TenantTags, func(i *resources.ReferenceDataItem) string { return i.ID })
		if err != nil {
			return err
		}

		flags.StepScopes.Value, err = PromptScope(asker, "Step", projectVariables.ScopeValues.Actions, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func PromptScope(ask question.Asker, scopeDescription string, items []*resources.ReferenceDataItem, displaySelector func(i *resources.ReferenceDataItem) string) ([]string, error) {
	if displaySelector == nil {
		displaySelector = func(i *resources.ReferenceDataItem) string { return i.Name }
	}
	if util.Empty(items) {
		return nil, nil
	}
	var selectedItems []string
	err := ask(&survey.MultiSelect{
		Message: fmt.Sprintf("%s scope", scopeDescription),
		Options: util.SliceTransform(items, displaySelector),
	}, &selectedItems)

	if err != nil {
		return nil, err
	}

	return selectedItems, nil
}
