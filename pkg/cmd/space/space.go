package space

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/space/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/space/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/space/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/space/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSpace(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space <command>",
		Short: "Manage spaces",
		Long:  "Manage spaces in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s space list
			$ %[1]s space view Spaces-302
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))

	return cmd
}
