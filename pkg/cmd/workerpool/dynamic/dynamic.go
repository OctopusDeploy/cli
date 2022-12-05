package dynamic

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/dynamic/create"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/dynamic/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdSsh(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dynamic <command>",
		Short:   "Manage dynamic worker pools",
		Long:    "Manage dynamic worker pools in Octopus Deploy",
		Example: heredoc.Docf("$ %s worker-pool dynamic view", constants.ExecutableName),
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
