package workerpool

import (
	"github.com/MakeNowJust/heredoc/v2"
	listCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdWorkerPool(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker-pool <command>",
		Short: "Manage worker pools",
		Long:  "Manage workers in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s worker list
			$ %[1]s worker ls
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(listCmd.NewCmdList(f))

	return cmd
}
