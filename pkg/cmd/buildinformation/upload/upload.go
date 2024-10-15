package upload

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/buildinformation"
	"github.com/spf13/cobra"
)

const (
	FlagPackageId = "package-id"
	FlagVersion   = "version"
	FlagFile      = "file"

	FlagOverwriteMode      = "overwrite-mode"
	FlagAliasOverwrite     = "overwrite"
	FlagAliasOverwriteMode = "overwritemode" // I keep forgetting the hyphen
)

type UploadFlags struct {
	PackageId     *flag.Flag[[]string]
	Version       *flag.Flag[string]
	File          *flag.Flag[string]
	OverwriteMode *flag.Flag[string]
}

func NewUploadFlags() *UploadFlags {
	return &UploadFlags{
		PackageId:     flag.New[[]string](FlagPackageId, false),
		Version:       flag.New[string](FlagVersion, false),
		File:          flag.New[string](FlagFile, false),
		OverwriteMode: flag.New[string](FlagOverwriteMode, false),
	}
}

type UploadOptions struct {
	*cmd.Dependencies
	*UploadFlags
}

func NewUploadOptions(uploadFlags *UploadFlags, dependencies *cmd.Dependencies) *UploadOptions {
	return &UploadOptions{
		UploadFlags:  uploadFlags,
		Dependencies: dependencies,
	}
}

func NewCmdUpload(f factory.Factory) *cobra.Command {
	uploadFlags := NewUploadFlags()
	cmd := &cobra.Command{
		Use:     "upload",
		Short:   "upload build information for one or more packages to Octopus Deploy",
		Long:    "upload build information one or more packages to Octopus Deploy.",
		Aliases: []string{"push"},
		Example: heredoc.Docf(`
			$ %[1]s build-information upload --package-id SomePackage --version 1.0.0 --file buildinfo.octopus
			$ %[1]s build-information upload SomePackage --version 1.0.0 --file buildinfo.octopus --overwrite-mode overwrite
			$ %[1]s build-information push SomePackage --version 1.0.0 --file buildinfo.octopus
			$ %[1]s build-information upload PkgA PkgB PkgC --version 1.0.0 --file buildinfo.octopus
		`, constants.ExecutableName),
		Annotations: map[string]string{annotations.IsCore: "true"},
		RunE: func(c *cobra.Command, args []string) error {
			// any bare args are assumed to be package ids to upload build information for
			uploadFlags.PackageId.Value = append(uploadFlags.PackageId.Value, args...)

			opts := NewUploadOptions(uploadFlags, cmd.NewDependencies(f, c))
			return uploadRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&uploadFlags.PackageId.Value, uploadFlags.PackageId.Name, "p", nil, "The ID of the package, may be specified multiple times. Any arguments without flags will be treated as package IDs")
	flags.StringVarP(&uploadFlags.Version.Value, uploadFlags.Version.Name, "", "", "The version of the package")
	flags.StringVarP(&uploadFlags.File.Value, uploadFlags.File.Name, "", "", "Path to Octopus Build Information Json file")
	flags.StringVarP(&uploadFlags.OverwriteMode.Value, uploadFlags.OverwriteMode.Name, "", "", "Action when a build information already exists. Valid values are 'fail', 'overwrite', 'ignore'. Default is 'fail'")
	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagOverwriteMode, flagAliases, FlagAliasOverwrite, FlagAliasOverwriteMode)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

func uploadRun(opts *UploadOptions) error {
	if !opts.NoPrompt {
		// Prompt for missing arguments
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	overwriteMode := opts.OverwriteMode.Value
	resolvedOverwriteMode := buildinformation.OverwriteMode("")
	switch strings.ToLower(overwriteMode) {
	case "fail", "failifexists", "": // include aliases from old CLI, default (empty string) = fail
		resolvedOverwriteMode = buildinformation.OverwriteModeFailIfExists
	case "ignore", "ignoreifexists":
		resolvedOverwriteMode = buildinformation.OverwriteModeIgnoreIfExists
	case "overwrite", "overwriteexisting", "replace":
		resolvedOverwriteMode = buildinformation.OverwriteModeOverwriteExisting
	default:
		return fmt.Errorf("invalid value '%s' for --overwrite-mode. Valid values are 'fail', 'ignore', 'overwrite'", overwriteMode)
	}

	var buildInformation buildinformation.OctopusBuildInformation
	if opts.File.Value != "" {
		jsonFile, err := os.ReadFile(opts.File.Value)
		if err != nil {
			return err
		}

		err = json.Unmarshal(jsonFile, &buildInformation)
		if err != nil {
			return err
		}

		fmt.Printf("Build information:\n%s\n", output.Dim(string(jsonFile)))
	}

	if len(buildInformation.Commits) >= 200 {
		fmt.Printf("%s\n", output.Yellow("Warning: Build information contains 200 or more commits, this may be due to a misconfiguration of your build server."))
	}

	for _, pkgIdString := range opts.PackageId.Value {
		cmd := buildinformation.NewCreateBuildInformationCommand(opts.Space.GetID(), pkgIdString, opts.Version.Value, buildInformation)
		cmd.OverwriteMode = resolvedOverwriteMode

		uploadedBuildInfo, err := buildinformation.Add(opts.Client, cmd)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(opts.Out, "\nSuccessfully uploaded build information for '%s' version '%s'  (%s).\n", uploadedBuildInfo.PackageID, uploadedBuildInfo.Version, uploadedBuildInfo.ID)
		if err != nil {
			return err
		}

		link := output.Bluef("%s/app#/%s/library/buildinformation/%s", opts.Host, opts.Space.GetID(), uploadedBuildInfo.GetID())
		fmt.Fprintf(opts.Out, "View this build information on Octopus Deploy: %s\n", link)
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.PackageId, opts.Version, opts.File, opts.OverwriteMode)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *UploadOptions) error {
	if len(opts.PackageId.Value) == 0 {
		var packageIdString string
		if err := opts.Ask(&survey.Multiline{
			Message: "Package ID(s)",
			Help:    "A multi-line list of Package IDs.",
		}, &packageIdString, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
		opts.PackageId.Value = strings.Split(packageIdString, "\n")
	}

	if opts.Version.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Version",
			Help:    "The version of the package.",
		}, &opts.Version.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.File.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Build information file",
			Help:    "Octopus Build Information JSON file.",
		}, &opts.File.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
		))); err != nil {
			return err
		}
	}

	return nil
}
