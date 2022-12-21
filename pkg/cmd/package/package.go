package _package

import (
	"fmt"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/package/list"
	cmdNuget "github.com/OctopusDeploy/cli/pkg/cmd/package/nuget"
	cmdUpload "github.com/OctopusDeploy/cli/pkg/cmd/package/upload"
	cmdVersions "github.com/OctopusDeploy/cli/pkg/cmd/package/versions"
	cmdZip "github.com/OctopusDeploy/cli/pkg/cmd/package/zip"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPackage(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package <command>",
		Short:   "Manage packages",
		Long:    "Manage packages in Octopus Deploy",
		Example: fmt.Sprintf("$ %s package upload", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdUpload.NewCmdUpload(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdVersions.NewCmdVersions(f))
	cmd.AddCommand(cmdNuget.NewCmdPackageNuget(f))
	cmd.AddCommand(cmdZip.NewCmdPackageZip(f))
	return cmd
}
