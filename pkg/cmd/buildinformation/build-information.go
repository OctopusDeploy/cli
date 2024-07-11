package buildinformation

import (
	"fmt"

	cmdBulkDelete "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/bulkdelete"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/list"
	cmdUpload "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/upload"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdBuildInformation(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "build-information <command>",
		Short:   "Manage build information",
		Long:    "Manage build information in Octopus Deploy",
		Example: fmt.Sprintf("$ %s build-information upload", constants.ExecutableName),
		Aliases: []string{"build-info"},
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdBulkDelete.NewCmdBulkDelete(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdUpload.NewCmdUpload(f))
	return cmd
}
