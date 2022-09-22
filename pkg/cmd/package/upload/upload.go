package upload

import (
	"errors"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

// TODO Outstanding questions for package upload:
// - What should the response be for outputformat basic and json?
// - Basic output: I have "Successfully uploaded package x-1.0.0.zip" when the package was created/overwritten, and "Successfully processed package x-1.0.0.zip" when it was ignored. Is this right?
// - If we specify multiple packages and one of them fails, does it keep going or abort on first error?
// - the package upload response from the server has a load of stuff in it. Is there any value in doing anything with this?

const (
	FlagPackage            = "package"
	FlagOverwriteMode      = "overwrite-mode"
	FlagAliasOverwrite     = "overwrite"
	FlagAliasOverwriteMode = "overwritemode" // I keep forgetting the hyphen
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
	uploadFlags := NewUploadFlags()
	cmd := &cobra.Command{
		Use:     "upload",
		Short:   "upload one or more packages to Octopus Deploy",
		Long:    "upload one or more packages to Octopus Deploy. Glob patterns are supported.",
		Aliases: []string{"push"},
		Example: heredoc.Docf(`
				$ %s package upload --package SomePackage.1.0.0.zip
				$ %s package upload SomePackage.1.0.0.tar.gz --overwrite-mode overwrite
				$ %s package push SomePackage.1.0.0.zip	
				$ %s package upload bin/**/*.zip --overwrite-mode ignore
				$ %s package upload PkgA.1.0.0.zip PkgB.2.0.0.tar.gz PkgC.1.0.0.nupkg
				`, constants.ExecutableName, constants.ExecutableName, constants.ExecutableName, constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{annotations.IsCore: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// any bare args are assumed to be packages to upload
			for _, arg := range args {
				uploadFlags.Package.Value = append(uploadFlags.Package.Value, arg)
			}

			return uploadRun(cmd, f, uploadFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&uploadFlags.Package.Value, uploadFlags.Package.Name, "p", nil, "Package to upload, may be specified multiple times.")
	flags.StringVarP(&uploadFlags.OverwriteMode.Value, uploadFlags.OverwriteMode.Name, "", "", "Action when a package already exists. Valid values are 'fail', 'overwrite', 'ignore'. Default is 'fail'")

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagOverwriteMode, flagAliases, FlagAliasOverwrite, FlagAliasOverwriteMode)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

func uploadRun(cmd *cobra.Command, f factory.Factory, flags *UploadFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	// package upload doesn't have interactive mode, so we don't care about the question.asker
	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}
	// core infrastructure will ask the user for a space interactively, should it need to
	space := f.GetCurrentSpace()
	if space == nil {
		return errors.New("package upload must run with a configured space")
	}

	template := packages.NewPackageUploadCommand(space.ID)

	overwriteMode := flags.OverwriteMode.Value
	switch strings.ToLower(overwriteMode) {
	case "fail", "failifexists", "": // include aliases from old CLI, default (empty string) = fail
		template.OverwriteMode = packages.OverwriteModeFailIfExists
	case "ignore", "ignoreifexists":
		template.OverwriteMode = packages.OverwriteModeIgnoreIfExists
	case "overwrite", "overwriteexisting", "replace":
		template.OverwriteMode = packages.OverwriteModeOverwriteExisting
	default:
		return fmt.Errorf("invalid value '%s' for --overwrite-mode. Valid values are 'fail', 'ignore', 'overwrite'", overwriteMode)
	}

	// with globs it's easy to upload the same thing twice by accident, so keep track of what's been uploaded as we go
	seenPackages := make(map[string]bool)
	for _, p := range flags.Package.Value {
		matches, err := filepath.Glob(p)
		// nil, nil means this wasn't a valid glob pattern; assume it's just a filepath
		if err == nil && matches == nil {
			if !seenPackages[p] {
				_, err := doUpload(octopus, template, p, cmd, outputFormat)
				if err != nil {
					return err
				}
				seenPackages[p] = true
			}
		} else if err != nil { // invalid glob pattern
			return err
		} else { // glob matched at least 1 thing
			for _, m := range matches {
				if !seenPackages[m] {
					_, err := doUpload(octopus, template, m, cmd, outputFormat)
					if err != nil {
						return err
					}
					seenPackages[m] = true
				}
			}
		}
	}
	return nil
}

func doUpload(octopus newclient.Client, uploadTemplate *packages.PackageUploadCommand, path string, cmd *cobra.Command, outputFormat string) (bool, error) {
	up := *uploadTemplate
	up.FileName = path

	var err error
	up.Contents, err = os.ReadFile(path)
	if err != nil {
		return false, err
	}

	_, created, err := packages.Upload(octopus, &up)

	if !constants.IsProgrammaticOutputFormat(outputFormat) {
		if created {
			cmd.Printf("Successfully uploaded package %s\n", path)
		} else {
			cmd.Printf("Successfully processed package %s\n", path)
		}
	}

	return created, err
}
