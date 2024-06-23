package project

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdBranch "github.com/OctopusDeploy/cli/pkg/cmd/project/branch"
	cmdClone "github.com/OctopusDeploy/cli/pkg/cmd/project/clone"
	cmdConnect "github.com/OctopusDeploy/cli/pkg/cmd/project/connect"
	cmdConvert "github.com/OctopusDeploy/cli/pkg/cmd/project/convert"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/project/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/project/delete"
	cmdDisconnect "github.com/OctopusDeploy/cli/pkg/cmd/project/disconnect"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/project/list"
	cmdVariables "github.com/OctopusDeploy/cli/pkg/cmd/project/variables"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/project/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdProject(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project <command>",
		Aliases: []string{"proj"},
		Short:   "Manage projects",
		Long:    "Manage projects in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project list
			$ %[1]s project ls
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdDelete.NewCmdList(f))
	cmd.AddCommand(cmdConnect.NewCmdConnect(f))
	cmd.AddCommand(cmdDisconnect.NewCmdDisconnect(f))
	cmd.AddCommand(cmdConvert.NewCmdConvert(f))
	cmd.AddCommand(cmdVariables.NewCmdVariables(f))
	cmd.AddCommand(cmdClone.NewCmdClone(f))
	cmd.AddCommand(cmdBranch.NewCmdBranch(f))

	return cmd
}
