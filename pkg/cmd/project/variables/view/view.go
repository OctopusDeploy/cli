package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	variableShared "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"io"
	"strconv"
	"strings"
)

const (
	FlagProject = "project"
	FlagWeb     = "web"
	FlagId      = "id"
)

type ViewFlags struct {
	Id      *flag.Flag[string]
	Project *flag.Flag[string]
	Web     *flag.Flag[bool]
}

type ViewOptions struct {
	Client *client.Client
	Host   string
	out    io.Writer
	name   string
	*ViewFlags
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

func viewRun(opts *ViewOptions) error {
	project, err := opts.Client.Projects.GetByIdentifier(opts.Project.Value)
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
			if opts.Id.Value != "" {
				return strings.EqualFold(variable.Name, opts.name) && strings.EqualFold(variable.ID, opts.Id.Value)
			}

			return strings.EqualFold(variable.Name, opts.name)
		})

	if !util.Any(filteredVars) {
		return fmt.Errorf("cannot find variable '%s'", opts.name)
	}

	fmt.Fprintln(opts.out, output.Bold(filteredVars[0].Name))

	for _, v := range filteredVars {
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

		scopeValues, err := variableShared.ToScopeValues(v, allVars.ScopeValues)
		if err != nil {
			return err
		}
		data = addScope(scopeValues.Environments, "Environment scope", data, nil)
		data = addScope(scopeValues.Roles, "Role scope", data, nil)
		data = addScope(scopeValues.Channels, "Channel scope", data, nil)
		data = addScope(scopeValues.Machines, "Machine scope", data, nil)
		data = addScope(scopeValues.TenantTags, "Tenant tag scope", data, func(item *resources.ReferenceDataItem) string {
			return item.ID
		})
		data = addScope(scopeValues.Actions, "Step scope", data, nil)
		data = addScope(
			util.SliceTransform(scopeValues.Processes, func(item *resources.ProcessReferenceDataItem) *resources.ReferenceDataItem {
				return &resources.ReferenceDataItem{
					ID:   item.ID,
					Name: item.Name,
				}
			}),
			"Process scope",
			data,
			nil)

		if v.Prompt != nil {
			data = append(data, output.NewDataRow("Prompted", "true"))
			data = append(data, output.NewDataRow("Prompt Label", v.Prompt.Label))
			data = append(data, output.NewDataRow("Prompt Description", output.Dim(v.Prompt.Description)))
			data = append(data, output.NewDataRow("Prompt Required", strconv.FormatBool(v.Prompt.IsRequired)))
		}

		fmt.Fprintln(opts.out)
		output.PrintRows(data, opts.out)
	}

	return nil
}

func addScope(values []*resources.ReferenceDataItem, scopeDescription string, data []*output.DataRow, displaySelector func(item *resources.ReferenceDataItem) string) []*output.DataRow {
	if displaySelector == nil {
		displaySelector = func(item *resources.ReferenceDataItem) string { return item.Name }
	}

	if util.Any(values) {
		data = append(data, output.NewDataRow(scopeDescription, output.FormatAsList(util.SliceTransform(values, displaySelector))))
	}

	return data
}
