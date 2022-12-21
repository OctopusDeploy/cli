package nuget

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdNugetCreate "github.com/OctopusDeploy/cli/pkg/cmd/package/nuget/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdPackageNuget(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "nuget <command>",
		Short:   "Package as NuPkg",
		Long:    "Package as NuPkg for Octopus Deploy",
		Example: heredoc.Docf("$ %s package nuget create", constants.ExecutableName),
	}

	cmd.AddCommand(cmdNugetCreate.NewCmdCreate(f))

	return cmd
}
