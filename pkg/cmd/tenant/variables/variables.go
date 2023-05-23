package variables

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/tenant/variables/list"
	cmdUpdate "github.com/OctopusDeploy/cli/pkg/cmd/tenant/variables/update"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdVariables(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "variables <command>",
		Aliases: []string{"variable"},
		Short:   "Manage tenant variables",
		Long:    "Manage tenant variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant variables list --tenant "Bobs Wood Shop"
			$ %[1]s tenant variables view --name "DatabaseName" --tenant "Bobs Wood Shop"
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	//cmd.AddCommand(cmdUpdate.NewUpdateCmd(f))
	//cmd.AddCommand(cmdCreate.NewCreateCmd(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdUpdate.NewCmdUpdate(f))
	//cmd.AddCommand(cmdView.NewCmdView(f))
	//cmd.AddCommand(cmdDelete.NewDeleteCmd(f))
	//cmd.AddCommand(cmdInclude.NewIncludeVariableSetCmd(f))
	//cmd.AddCommand(cmdExclude.NewExcludeVariableSetCmd(f))

	return cmd
}
