package workerpool

import (
	"github.com/MakeNowJust/heredoc/v2"
	deleteCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/delete"
	dynamicCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/dynamic"
	listCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/list"
	staticCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/static"
	viewCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdWorkerPool(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker-pool <command>",
		Short: "Manage worker pools",
		Long:  "Manage worker pools in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker-pool list
			$ %[1]s worker-pool ls
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(deleteCmd.NewCmdDelete(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(viewCmd.NewCmdView(f))
	cmd.AddCommand(staticCmd.NewCmdStatic(f))
	cmd.AddCommand(dynamicCmd.NewCmdSsh(f))

	return cmd
}
