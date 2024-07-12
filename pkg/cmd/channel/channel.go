package channel

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/channel/create"
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
			$ %[1]s channel create
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
