package channel

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/channel/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/channel/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/channel/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/channel/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdChannel(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel <command>",
		Short: "Manage channels",
		Long:  "Manage channels in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s channel create
			%[1]s channel list --project myProject
			%[1]s channel view "Hotfix" --project myProject
			%[1]s channel delete "Hotfix" --project myProject
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))

	return cmd
}
