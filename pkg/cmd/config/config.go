package config

import (
	getCmd "github.com/OctopusDeploy/cli/pkg/cmd/config/get"
	listCmd "github.com/OctopusDeploy/cli/pkg/cmd/config/list"
	setCmd "github.com/OctopusDeploy/cli/pkg/cmd/config/set"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdConfig(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
		Short: "Manage CLI configuration",
		Long:  "Manage the CLI configuration",
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	cmd.AddCommand(getCmd.NewCmdGet(f))
	cmd.AddCommand(setCmd.NewCmdSet(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	return cmd
}
