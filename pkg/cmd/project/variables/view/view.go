package view

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	variableShared "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagId      = "id"
	FlagWeb     = "web"
)

type ViewFlags struct {
	Id      *flag.Flag[string]
	Project *flag.Flag[string]
	Web     *flag.Flag[bool]
}

type ViewOptions struct {
	Client  *client.Client
	Host    string
	out     io.Writer
	name    string
	flags   *ViewFlags
	Command *cobra.Command
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Project: flag.New[string](FlagProject, false),
		Id:      flag.New[string](FlagId, false),
		Web:     flag.New[bool](FlagWeb, false),
	}
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View all values of a project variable",
		Long:  "View all values of a project variable in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable view
			$ %[1]s project variable view DatabaseName --project "Vet Clinic"
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must supply variable name")
			}

			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				cmd.OutOrStdout(),
				args[0],
				viewFlags,
				cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&viewFlags.Project.Value, viewFlags.Project.Name, "p", "", "The project containing the variable")
	flags.StringVar(&viewFlags.Id.Value, viewFlags.Id.Name, "", "The Id of the specifically scoped variable")
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

type VariableValueWithScope struct {
	Variable     *variables.Variable
	ScopeValues  *variables.VariableScopeValues
	VariableName string
	Project      *projects.Project
}

func viewRun(opts *ViewOptions) error {
	project, err := opts.Client.Projects.GetByIdentifier(opts.flags.Project.Value)
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
			if opts.flags.Id.Value != "" {
				return strings.EqualFold(variable.Name, opts.name) && strings.EqualFold(variable.ID, opts.flags.Id.Value)
			}

			return strings.EqualFold(variable.Name, opts.name)
		})

	if !util.Any(filteredVars) {
		return fmt.Errorf("cannot find variable '%s'", opts.name)
	}

	// Build enriched variable values with scope information
	var enrichedVars []*VariableValueWithScope
	for _, v := range filteredVars {
		scopeValues, err := variableShared.ToScopeValues(v, allVars.ScopeValues)
		if err != nil {
			return err
		}
		enrichedVars = append(enrichedVars, &VariableValueWithScope{
			Variable:     v,
			ScopeValues:  scopeValues,
			VariableName: filteredVars[0].Name,
			Project:      project,
		})
	}

	return output.PrintArray(enrichedVars, opts.Command, output.Mappers[*VariableValueWithScope]{
		Json: func(item *VariableValueWithScope) any {
			return getVariableValueAsJson(opts, item)
		},
		Table: output.TableDefinition[*VariableValueWithScope]{
			Header: []string{"ID", "VALUE", "DESCRIPTION", "SCOPES"},
			Row: func(item *VariableValueWithScope) []string {
				return getVariableValueAsTableRow(item)
			},
		},
		Basic: func(item *VariableValueWithScope) string {
			return formatVariableValueForBasic(opts, item)
		},
	})
}

type VariableValueAsJson struct {
	Id           string                           `json:"Id"`
	Name         string                           `json:"Name"`
	Value        string                           `json:"Value"`
	IsSensitive  bool                             `json:"IsSensitive"`
	Description  string                           `json:"Description"`
	Scope        *variables.VariableScopeValues   `json:"Scope"`
	Prompt       *variables.VariablePromptOptions `json:"Prompt,omitempty"`
	WebUrl       string                           `json:"WebUrl"`
}

func getVariableValueAsJson(opts *ViewOptions, item *VariableValueWithScope) VariableValueAsJson {
	description := item.Variable.Description
	if description == "" {
		description = constants.NoDescription
	}

	return VariableValueAsJson{
		Id:          item.Variable.GetID(),
		Name:        item.VariableName,
		Value:       item.Variable.Value,
		IsSensitive: item.Variable.IsSensitive,
		Description: description,
		Scope:       item.ScopeValues,
		Prompt:      item.Variable.Prompt,
		WebUrl:      util.GenerateWebURL(opts.Host, item.Project.SpaceID, fmt.Sprintf("projects/%s/variables", item.Project.Slug)),
	}
}

func getVariableValueAsTableRow(item *VariableValueWithScope) []string {
	v := item.Variable
	scopeValues := item.ScopeValues

	var value string
	if v.IsSensitive {
		value = "*** (sensitive)"
	} else {
		value = v.Value
	}

	description := v.Description
	if description == "" {
		description = constants.NoDescription
	}

	// Build scope summary
	var scopeParts []string
	if util.Any(scopeValues.Environments) {
		scopeParts = append(scopeParts, fmt.Sprintf("Env: %s", formatScopeList(scopeValues.Environments, nil)))
	}
	if util.Any(scopeValues.Roles) {
		scopeParts = append(scopeParts, fmt.Sprintf("Role: %s", formatScopeList(scopeValues.Roles, nil)))
	}
	if util.Any(scopeValues.Channels) {
		scopeParts = append(scopeParts, fmt.Sprintf("Channel: %s", formatScopeList(scopeValues.Channels, nil)))
	}
	if util.Any(scopeValues.Machines) {
		scopeParts = append(scopeParts, fmt.Sprintf("Machine: %s", formatScopeList(scopeValues.Machines, nil)))
	}
	if util.Any(scopeValues.TenantTags) {
		scopeParts = append(scopeParts, fmt.Sprintf("Tag: %s", formatScopeList(scopeValues.TenantTags, func(item *resources.ReferenceDataItem) string {
			return item.ID
		})))
	}
	if util.Any(scopeValues.Actions) {
		scopeParts = append(scopeParts, fmt.Sprintf("Step: %s", formatScopeList(scopeValues.Actions, nil)))
	}
	if util.Any(scopeValues.Processes) {
		scopeParts = append(scopeParts, fmt.Sprintf("Process: %s", formatProcessList(scopeValues.Processes)))
	}

	scopes := strings.Join(scopeParts, "; ")
	if scopes == "" {
		scopes = "No scopes"
	}

	return []string{
		output.Dim(v.GetID()),
		value,
		description,
		scopes,
	}
}

func formatVariableValueForBasic(opts *ViewOptions, item *VariableValueWithScope) string {
	v := item.Variable
	scopeValues := item.ScopeValues

	data := []*output.DataRow{}

	data = append(data, output.NewDataRow("Id", output.Dim(v.GetID())))
	if v.IsSensitive {
		data = append(data, output.NewDataRow("Value", output.Bold("*** (sensitive)")))
	} else {
		data = append(data, output.NewDataRow("Value", output.Bold(v.Value)))
	}

	if v.Description == "" {
		v.Description = constants.NoDescription
	}
	data = append(data, output.NewDataRow("Description", output.Dim(v.Description)))

	if util.Any(scopeValues.Environments) {
		data = append(data, output.NewDataRow("Environment scope", output.FormatAsList(util.SliceTransform(scopeValues.Environments, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}
	if util.Any(scopeValues.Roles) {
		data = append(data, output.NewDataRow("Role scope", output.FormatAsList(util.SliceTransform(scopeValues.Roles, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}
	if util.Any(scopeValues.Channels) {
		data = append(data, output.NewDataRow("Channel scope", output.FormatAsList(util.SliceTransform(scopeValues.Channels, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}
	if util.Any(scopeValues.Machines) {
		data = append(data, output.NewDataRow("Machine scope", output.FormatAsList(util.SliceTransform(scopeValues.Machines, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}
	if util.Any(scopeValues.TenantTags) {
		data = append(data, output.NewDataRow("Tenant tag scope", output.FormatAsList(util.SliceTransform(scopeValues.TenantTags, func(item *resources.ReferenceDataItem) string { return item.ID }))))
	}
	if util.Any(scopeValues.Actions) {
		data = append(data, output.NewDataRow("Step scope", output.FormatAsList(util.SliceTransform(scopeValues.Actions, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}
	if util.Any(scopeValues.Processes) {
		data = append(data, output.NewDataRow("Process scope", output.FormatAsList(util.SliceTransform(scopeValues.Processes, func(item *resources.ProcessReferenceDataItem) string { return item.Name }))))
	}

	if v.Prompt != nil {
		data = append(data, output.NewDataRow("Prompted", "true"))
		data = append(data, output.NewDataRow("Prompt Label", v.Prompt.Label))
		data = append(data, output.NewDataRow("Prompt Description", output.Dim(v.Prompt.Description)))
		data = append(data, output.NewDataRow("Prompt Required", strconv.FormatBool(v.Prompt.IsRequired)))
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n\n", output.Bold(item.VariableName))
	output.PrintRows(data, &buf)

	url := util.GenerateWebURL(opts.Host, item.Project.SpaceID, fmt.Sprintf("projects/%s/variables", item.Project.Slug))
	fmt.Fprintf(&buf, "\nView this project's variables in Octopus Deploy: %s\n", output.Blue(url))

	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}

	return buf.String()
}

func formatScopeList(values []*resources.ReferenceDataItem, displaySelector func(item *resources.ReferenceDataItem) string) string {
	if displaySelector == nil {
		displaySelector = func(item *resources.ReferenceDataItem) string { return item.Name }
	}
	return strings.Join(util.SliceTransform(values, displaySelector), ", ")
}

func formatProcessList(processes []*resources.ProcessReferenceDataItem) string {
	return strings.Join(util.SliceTransform(processes, func(item *resources.ProcessReferenceDataItem) string {
		return item.Name
	}), ", ")
}
