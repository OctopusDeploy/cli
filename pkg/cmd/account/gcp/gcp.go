package gcp

import (
	"fmt"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/create"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdGcp(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gcp <command>",
		Short:   "Manage GCP accounts",
		Long:    `Work with Octopus Deploy Google Cloud accounts.`,
		Example: fmt.Sprintf("$ %s account gcp list", constants.ExecutableName),
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))

	return cmd
}
