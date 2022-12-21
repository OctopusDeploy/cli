package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/variables/scopes"
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
	"strings"
)

const (
	FlagProject = "project"
	FlagWeb     = "web"
)

type ViewFlags struct {
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

		scopeValues, err := scopes.ToScopeValues(v, allVars.ScopeValues)
		if err != nil {
			return err
		}
		data = addScope(scopeValues.Environments, "Environment scope", data)
		data = addScope(scopeValues.Roles, "Role scope", data)
		data = addScope(scopeValues.Channels, "Channel scope", data)
		data = addScope(scopeValues.Machines, "Machine scope", data)
		data = addScope(scopeValues.TenantTags, "Tenant tag scope", data)
		data = addScope(scopeValues.Actions, "Step scope", data)
		data = addScope(
			util.SliceTransform(scopeValues.Processes, func(item *resources.ProcessReferenceDataItem) *resources.ReferenceDataItem {
				return &resources.ReferenceDataItem{
					ID:   item.ID,
					Name: item.Name,
				}
			}),
			"Process scope",
			data)

		fmt.Fprintln(opts.out)
		output.PrintRows(data, opts.out)
	}

	return nil
}

func addScope(values []*resources.ReferenceDataItem, scopeDescription string, data []*output.DataRow) []*output.DataRow {
	if util.Any(values) {
		data = append(data, output.NewDataRow(scopeDescription, output.FormatAsList(util.SliceTransform(values, func(item *resources.ReferenceDataItem) string { return item.Name }))))
	}

	return data
}
