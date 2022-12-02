package config

import (
	waitCmd "github.com/OctopusDeploy/cli/pkg/cmd/task/wait"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTask(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task <command>",
		Short: "Manage tasks",
		Long:  "Manage tasks in Octopus Deploy",
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(waitCmd.NewCmdWait(f))

	return cmd
}
