package create

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	FlagAuthor           = "author"
	FlagTitle            = "title"
	FlagDescription      = "description"
	FlagReleaseNotes     = "releaseNotes"
	FlagReleaseNotesFile = "releaseNotesFile"
)

type NuPkgCreateFlags struct {
	*pack.PackageCreateFlags
	Author           *flag.Flag[[]string] // this need to be multiple and default to current user
	Title            *flag.Flag[string]
	Description      *flag.Flag[string]
	ReleaseNotes     *flag.Flag[string]
	ReleaseNotesFile *flag.Flag[string]
}

type NuPkgCreateOptions struct {
	*NuPkgCreateFlags
	*pack.PackageCreateOptions
}

func NewNuPkgCreateFlags() *NuPkgCreateFlags {
	return &NuPkgCreateFlags{
		PackageCreateFlags: pack.NewPackageCreateFlags(),
		Author:             flag.New[[]string](FlagAuthor, false),
		Title:              flag.New[string](FlagTitle, false),
		Description:        flag.New[string](FlagDescription, false),
		ReleaseNotes:       flag.New[string](FlagReleaseNotes, false),
		ReleaseNotesFile:   flag.New[string](FlagReleaseNotesFile, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewNuPkgCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create nuget",
		Long:  "Create nuget package",
		Example: heredoc.Docf(`
			$ %[1]s project nuget create --id SomePackage --version 1.0.0
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &NuPkgCreateOptions{
				NuPkgCreateFlags:     createFlags,
				PackageCreateOptions: pack.NewPackageCreateOptions(f, createFlags.PackageCreateFlags, cmd),
			}
			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&createFlags.Id.Value, createFlags.Id.Name, "", "The ID of the package")
	flags.StringVarP(&createFlags.Version.Value, createFlags.Version.Name, "v", "", "The version of the package; must be a valid SemVer; defaults to a timestamp-based version")
	flags.StringVar(&createFlags.BasePath.Value, createFlags.BasePath.Name, ".", "Root folder containing the contents to zip")
	flags.StringVar(&createFlags.OutFolder.Value, createFlags.OutFolder.Name, ".", "Folder into which the zip file will be written")
	flags.StringSliceVar(&createFlags.Include.Value, createFlags.Include.Name, []string{"**"}, "Add a file pattern to include, relative to the base path e.g. /bin/*.dll")
	flags.BoolVar(&createFlags.Verbose.Value, createFlags.Verbose.Name, false, "Verbose output")
	flags.BoolVar(&createFlags.Overwrite.Value, createFlags.Overwrite.Name, false, "Allow an existing package file of the same ID/version to be overwritten")
	flags.StringSliceVar(&createFlags.Author.Value, createFlags.Author.Name, []string{}, "Add author/s to the package metadata; defaults to the current user")
	flags.StringVar(&createFlags.Title.Value, createFlags.Title.Name, "", "The title of the package")
	flags.StringVar(&createFlags.Description.Value, createFlags.Description.Name, "A deployment package created from files on disk.", "A description of the package")
	flags.StringVar(&createFlags.ReleaseNotes.Value, createFlags.ReleaseNotes.Name, "", "Release notes for this version of the package")
	flags.StringVar(&createFlags.ReleaseNotesFile.Value, createFlags.ReleaseNotesFile.Name, "", "A file containing release notes for this version of the package")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *NuPkgCreateOptions) error {
	//if !opts.NoPrompt {
	//	if err := PromptMissing(opts); err != nil {
	//		return err
	//	}
	//}

	packOpts := opts.PackageCreateOptions

	if packOpts.Id.Value == "" {
		return errors.New("must supply a package ID")
	}

	if packOpts.Version.Value == "" {
		packOpts.Version.Value = pack.BuildTimestampSemVer(time.Now())
	}

	nuspecFilePath := filepath.Join(packOpts.BasePath.Value, packOpts.Id.Value+".nuspec")
	err := GenerateNuSpec(opts, nuspecFilePath)
	if err != nil {
		return err
	}

	pack.VerboseOut(packOpts.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", packOpts.Id.Value, packOpts.Version.Value)

	opts.PackageCreateOptions.Include.Value = append(packOpts.Include.Value, packOpts.Id.Value+".nuspec")
	outFilePath := pack.BuildOutFileName("nupkg", packOpts.Id.Value, packOpts.Version.Value)

	err = pack.BuildPackage(packOpts, outFilePath)
	if err != nil {
		return err
	}

	return os.Remove(nuspecFilePath)
}

func GenerateNuSpec(opts *NuPkgCreateOptions, filePath string) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<package xmlns="http://schemas.microsoft.com/packaging/2010/07/nuspec.xsd">` + "\n")
	sb.WriteString("  <metadata>\n")
	sb.WriteString("    <id>" + opts.PackageCreateOptions.Id.Value + "</id>\n")
	sb.WriteString("    <version>" + opts.PackageCreateOptions.Version.Value + "</version>\n")
	sb.WriteString("    <description>" + opts.Description.Value + "</description>\n")
	sb.WriteString("    <authors>" + strings.Join(opts.Author.Value, ",") + "</authors>\n")
	if opts.ReleaseNotes.Value != "" {
		sb.WriteString("    <releaseNotes>" + opts.ReleaseNotes.Value + "</releaseNotes>\n")
	}
	sb.WriteString("  </metadata>\n")
	sb.WriteString("</package>\n")

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	_, err = file.WriteString(sb.String())
	if err != nil {
		return err
	}

	return file.Close()
}
