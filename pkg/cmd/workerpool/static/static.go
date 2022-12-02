package static

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/static/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdStatic(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "static <command>",
		Short:   "Manage static worker pools",
		Long:    "Manage static worker pools in Octopus Deploy",
		Example: heredoc.Docf("$ %s worker-pool static view", constants.ExecutableName),
	}

	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
