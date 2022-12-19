package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command{
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

func listRun(cmd *cobra.Command, f factory.Factory, id string) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	project, err := client.Projects.GetByIdentifier(id)
	if err != nil { return err }

	vars, err := client.Variables.GetByID(project.GetID(), project.VariableSetID)
	if err != nil { return err }



	return output.PrintArray(vars, cmd, output.Mappers[*variables.Variable]{
		Basic: func(v *variables.Variable) {
			return v.Name
		},f,
	}

}
