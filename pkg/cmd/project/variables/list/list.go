package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/variables/scopes"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"strconv"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project variables",
		Long:  "List project variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable list
			$ %[1]s project variable ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must supply project identifier")
			}
			return listRun(cmd, f, args[0])
		},
	}

	return cmd
}

type VariableAsJson struct {
	*variables.Variable
	Scope variables.VariableScopeValues
}

func listRun(cmd *cobra.Command, f factory.Factory, id string) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	project, err := client.Projects.GetByIdentifier(id)
	if err != nil {
		return err
	}

	vars, err := client.Variables.GetAll(project.GetID())
	if err != nil {
		return err
	}

	return output.PrintArray(vars.Variables, cmd, output.Mappers[*variables.Variable]{
		Json: func(v *variables.Variable) any {
			enhancedScope, err := scopes.ToScopeValues(v, vars)
			if err != nil {
				return err
			}
			return VariableAsJson{
				Variable: v,
				Scope:    *enhancedScope}
		},
		Table: output.TableDefinition[*variables.Variable]{
			Header: []string{"NAME", "DESCRIPTION", "VALUE", "IS PROMPTED", "ID"},
			Row: func(v *variables.Variable) []string {
				return []string{output.Bold(v.Name), v.Description, getValue(v), strconv.FormatBool(v.Prompt != nil), output.Dim(v.GetID())}
			},
		},
		Basic: func(v *variables.Variable) string {
			return v.Name
		},
	})

}

func getValue(v *variables.Variable) string {
	if v.IsSensitive {
		return "***"
	}

	return v.Value
}
