package _package

import (
	"fmt"
	cmdUpload "github.com/OctopusDeploy/cli/pkg/cmd/package/upload"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPackage(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package <command>",
		Short:   "Manage packages",
		Long:    `Work with Octopus Deploy packages.`,
		Example: fmt.Sprintf("$ %s package upload", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdUpload.NewCmdUpload(f))
	return cmd
}
