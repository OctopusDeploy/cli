package projectgroup

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	createCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	deleteCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/delete"
	listCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/list"
	viewCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdProjectGroup(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project-group <command>",
		Short: "Manage project groups",
		Long:  `Work with Octopus Deploy project groups.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s project-group list
			$ %s project-group ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(createCmd.NewCmdCreate(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(deleteCmd.NewCmdList(f))
	cmd.AddCommand(viewCmd.NewCmdView(f))

	return cmd
}
