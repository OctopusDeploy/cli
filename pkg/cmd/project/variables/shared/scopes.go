package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strings"
)

const (
	FlagEnvironmentScope = "environment-scope"
	FlagTargetScope      = "target-scope"
	FlagStepScope        = "step-scope"
	FlagRoleScope        = "role-scope"
	FlagChannelScope     = "channel-scope"
	FlagTagScope         = "tag-scope"
	FlagProcessScope     = "process-scope"
)

type ScopeFlags struct {
	EnvironmentsScopes *flag.Flag[[]string]
	ChannelScopes      *flag.Flag[[]string]
	TargetScopes       *flag.Flag[[]string]
	StepScopes         *flag.Flag[[]string]
	RoleScopes         *flag.Flag[[]string]
	TagScopes          *flag.Flag[[]string]
	ProcessScopes      *flag.Flag[[]string]
}

func NewScopeOptions() *ScopeFlags {
	return &ScopeFlags{
		EnvironmentsScopes: flag.New[[]string](FlagEnvironmentScope, false),
		ChannelScopes:      flag.New[[]string](FlagChannelScope, false),
		TargetScopes:       flag.New[[]string](FlagTargetScope, false),
		StepScopes:         flag.New[[]string](FlagStepScope, false),
		RoleScopes:         flag.New[[]string](FlagRoleScope, false),
		TagScopes:          flag.New[[]string](FlagTagScope, false),
		ProcessScopes:      flag.New[[]string](FlagProcessScope, false),
	}
}

func RegisterScopeFlags(cmd *cobra.Command, scopeFlags *ScopeFlags) {
	flags := cmd.Flags()
	flags.StringSliceVar(&scopeFlags.EnvironmentsScopes.Value, scopeFlags.EnvironmentsScopes.Name, []string{}, "Assign environment scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.ChannelScopes.Value, scopeFlags.ChannelScopes.Name, []string{}, "Assign channel scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.TargetScopes.Value, scopeFlags.TargetScopes.Name, []string{}, "Assign deployment target scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.StepScopes.Value, scopeFlags.StepScopes.Name, []string{}, "Assign process step scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.RoleScopes.Value, scopeFlags.RoleScopes.Name, []string{}, "Assign role scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.TagScopes.Value, scopeFlags.TagScopes.Name, []string{}, "Assign tag scopes to the variable. Multiple scopes can be supplied.")
	flags.StringSliceVar(&scopeFlags.ProcessScopes.Value, scopeFlags.ProcessScopes.Name, []string{}, "Assign process scopes to the variable. Valid scopes are 'deployment' or a runbook name. Multiple scopes can be supplied.")
}

func ToScopeValues(variable *variables.Variable, variableScopeValues *variables.VariableScopeValues) (*variables.VariableScopeValues, error) {
	scopeValues := &variables.VariableScopeValues{}

	var err error
	scopeValues.Environments, err = getSingleScope(variable.Scope.Environments, variableScopeValues.Environments)
	if err != nil {
		return nil, err
	}

	scopeValues.Channels, err = getSingleScope(variable.Scope.Channels, variableScopeValues.Channels)
	if err != nil {
		return nil, err
	}

	scopeValues.Actions, err = getSingleScope(variable.Scope.Actions, variableScopeValues.Actions)
	if err != nil {
		return nil, err
	}

	scopeValues.TenantTags, err = getSingleScope(variable.Scope.TenantTags, variableScopeValues.TenantTags)
	if err != nil {
		return nil, err
	}

	scopeValues.Roles, err = getSingleScope(variable.Scope.Roles, variableScopeValues.Roles)
	if err != nil {
		return nil, err
	}

	scopeValues.Machines, err = getSingleScope(variable.Scope.Machines, variableScopeValues.Machines)
	if err != nil {
		return nil, err
	}

	scopeValues.Processes, err = getSingleProcessScope(variable.Scope.ProcessOwners, variableScopeValues.Processes)
	if err != nil {
		return nil, err
	}

	return scopeValues, nil
}

func getSingleScope(scopes []string, lookupScopes []*resources.ReferenceDataItem) ([]*resources.ReferenceDataItem, error) {
	var referenceScopes []*resources.ReferenceDataItem
	for _, s := range scopes {
		scope, err := findSingleScope(s, lookupScopes)
		if err != nil {
			return nil, err
		}
		referenceScopes = append(referenceScopes, scope)
	}

	return referenceScopes, nil
}

func findSingleScope(scope string, scopes []*resources.ReferenceDataItem) (*resources.ReferenceDataItem, error) {
	for _, s := range scopes {
		if strings.EqualFold(scope, s.ID) {
			return s, nil
		}
	}

	return nil, fmt.Errorf("cannot find scope value for '%s'", scope)
}

func getSingleProcessScope(scopes []string, lookupScopes []*resources.ProcessReferenceDataItem) ([]*resources.ProcessReferenceDataItem, error) {
	var referenceScopes []*resources.ProcessReferenceDataItem
	for _, s := range scopes {
		scope, err := findSingleProcessScope(s, lookupScopes)
		if err != nil {
			return nil, err
		}
		referenceScopes = append(referenceScopes, scope)
	}

	return referenceScopes, nil
}

func findSingleProcessScope(scope string, scopes []*resources.ProcessReferenceDataItem) (*resources.ProcessReferenceDataItem, error) {
	for _, s := range scopes {
		if strings.EqualFold(scope, s.ID) {
			return s, nil
		}
	}

	return nil, fmt.Errorf("cannot find scope value for '%s'", scope)
}

func ToVariableScope(projectVariables variables.VariableSet, opts *ScopeFlags, project *projects.Project) (*variables.VariableScope, error) {
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

	processScopeReference := ConvertProcessScopesToReference(projectVariables.ScopeValues.Processes)
	processScopeReference = append(processScopeReference, &resources.ReferenceDataItem{ID: project.GetID(), Name: "deployment"})
	scope.ProcessOwners, err = buildSingleScope(opts.ProcessScopes.Value, processScopeReference)
	if err != nil {
		return nil, err
	}

	return scope, nil
}

func ConvertProcessScopesToReference(processes []*resources.ProcessReferenceDataItem) []*resources.ReferenceDataItem {
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
