package variables

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/delete"
	cmdExclude "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/exclude"
	cmdInclude "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/include"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/list"
	cmdUpdate "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/update"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdVariables(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "variables <command>",
		Aliases: []string{"variable"},
		Short:   "Manage project variables",
		Long:    "Manage project variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable list "Deploy Web App"
			$ %[1]s project variable view --name "DatabaseName" --project Deploy
			$ %[1]s project variable update
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdUpdate.NewUpdateCmd(f))
	cmd.AddCommand(cmdCreate.NewCreateCmd(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewDeleteCmd(f))
	cmd.AddCommand(cmdInclude.NewIncludeVariableSetCmd(f))
	cmd.AddCommand(cmdExclude.NewExcludeVariableSetCmd(f))

	return cmd
}
