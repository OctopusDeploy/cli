package upload

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagPackage       = "package"
	FlagOverwriteMode = "overwrite-mode"
	// replace-existing deprected in the .NET CLI so not brought across
	FlagUseDeltaCompression = "use-delta-compression"
)

type UploadFlags struct {
	Package             *flag.Flag[[]string]
	OverwriteMode       *flag.Flag[string]
	UseDeltaCompression *flag.Flag[bool]
}

func NewUploadFlags() *UploadFlags {
	return &UploadFlags{
		Package:             flag.New[[]string](FlagPackage, false),
		OverwriteMode:       flag.New[string](FlagOverwriteMode, false),
		UseDeltaCompression: flag.New[bool](FlagUseDeltaCompression, false),
	}
}

func NewCmdUpload(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upload",
		Short:   "upload one or more packages to Octopus Deploy",
		Long:    "upload one or more packages to Octopus Deploy. Glob patterns are supported.",
		Aliases: []string{"push"},
		Example: heredoc.Docf(`
				$ %s package upload SomePackage.1.0.0.zip
				$ %s package push SomePackage.1.0.0.zip
				$ %s package upload --package SomePackage.1.0.0.zip
				$ %s package upload bin/**/*.zip
				$ %s package upload SomePackage.1.0.0.zip OtherPackage.2.0.0.tar.gz ThirdPackage.1.0.0.nupkg
				`, constants.ExecutableName),
		Annotations: map[string]string{annotations.IsCore: "true"},
	}
	return cmd
}
