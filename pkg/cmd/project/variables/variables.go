package variables

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/list"
	cmdSet "github.com/OctopusDeploy/cli/pkg/cmd/project/variables/set"
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
			$ %[1]s project variable set
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdSet.NewSetCmd(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewDeleteCmd(f))

	return cmd
}
