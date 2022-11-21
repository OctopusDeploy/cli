package release

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/release/delete"
	cmdDeploy "github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/release/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdRelease(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "release <command>",
		Short:   "Manage releases",
		Long:    "Manage releases in Octopus Deploy",
		Example: heredoc.Docf("$ %s release list", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdDeploy.NewCmdDeploy(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	return cmd
}
