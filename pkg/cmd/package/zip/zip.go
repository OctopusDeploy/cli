package zip

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdZipCreate "github.com/OctopusDeploy/cli/pkg/cmd/package/zip/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPackageZip(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "zip <command>",
		Short:   "Package as zip",
		Long:    "Package as zip for Octopus Deploy",
		Example: heredoc.Docf("$ %s package zip create", constants.ExecutableName),
	}

	cmd.AddCommand(cmdZipCreate.NewCmdCreate(f))

	return cmd
}
