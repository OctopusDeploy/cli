package platformhub

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdConfigure "github.com/OctopusDeploy/cli/pkg/cmd/platformhub/configure"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/platformhub/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPlatformHub(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platform-hub <command>",
		Short: "Manage Platform Hub",
		Long:  "Manage Platform Hub in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s platform-hub configure
			%[1]s platform-hub view
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	cmd.AddCommand(cmdConfigure.NewCmdConfigure(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
