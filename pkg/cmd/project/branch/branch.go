package branch

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/project/branch/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/project/branch/list"
)

func NewCmdBranch(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "branch <command>",
		Aliases: []string{"variable"},
		Short:   "Manage project branches",
		Long:    "Manage project branches in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project branch list "Deploy Web App"
			$ %[1]s project branch create -p "Deploy Web App" --new-branch-name add-name-variable --base-branch refs/heads/main -
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
