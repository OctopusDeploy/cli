package projectgroup

import (
	"github.com/MakeNowJust/heredoc/v2"
	createCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	deleteCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/delete"
	listCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/list"
	cmdListProjects "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/projects"
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
		Long:  "Manage project groups in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project-group list
			$ %[1]s project-group ls
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(createCmd.NewCmdCreate(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(deleteCmd.NewCmdList(f))
	cmd.AddCommand(viewCmd.NewCmdView(f))
	cmd.AddCommand(cmdListProjects.NewCmdListProjects(f))

	return cmd
}
