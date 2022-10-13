package project

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/project/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/project/list"
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
		Long:    `Work with Octopus Deploy projects.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s project list
			$ %s project ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewCmdList(f))

	return cmd
}
