package create

import (
	"errors"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
	"os"
	"os/user"
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
	flags.StringVar(&createFlags.BasePath.Value, createFlags.BasePath.Name, "", "Root folder containing the contents to zip")
	flags.StringVar(&createFlags.OutFolder.Value, createFlags.OutFolder.Name, "", "Folder into which the zip file will be written")
	flags.StringSliceVar(&createFlags.Include.Value, createFlags.Include.Name, []string{}, "Add a file pattern to include, relative to the base path e.g. /bin/*.dll; defaults to \"**\"")
	flags.BoolVar(&createFlags.Verbose.Value, createFlags.Verbose.Name, false, "Verbose output")
	flags.BoolVar(&createFlags.Overwrite.Value, createFlags.Overwrite.Name, false, "Allow an existing package file of the same ID/version to be overwritten")
	flags.StringSliceVar(&createFlags.Author.Value, createFlags.Author.Name, []string{}, "Add author/s to the package metadata; defaults to the current user")
	flags.StringVar(&createFlags.Title.Value, createFlags.Title.Name, "", "The title of the package")
	flags.StringVar(&createFlags.Description.Value, createFlags.Description.Name, "", "A description of the package")
	flags.StringVar(&createFlags.ReleaseNotes.Value, createFlags.ReleaseNotes.Name, "", "Release notes for this version of the package")
	flags.StringVar(&createFlags.ReleaseNotesFile.Value, createFlags.ReleaseNotesFile.Name, "", "A file containing release notes for this version of the package")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *NuPkgCreateOptions) error {
	if !opts.NoPrompt {
		if err := pack.PackageCreatePromptMissing(opts.PackageCreateOptions); err != nil {
			return err
		}
		if err := PromptMissing(opts); err != nil {
			return err
		}

	}

	if opts.PackageCreateOptions.Id.Value == "" {
		return errors.New("must supply a package ID")
	}

	err := applyDefaultsToUnspecifiedPackageOptions(opts)
	if err != nil {
		return err
	}

	var nuspecFilePath string
	if shouldGenerateNuSpec(opts) {
		nuspecFilePath, err = GenerateNuSpec(opts)
		if err != nil {
			return err
		}
		opts.PackageCreateOptions.Include.Value = append(opts.PackageCreateOptions.Include.Value, opts.PackageCreateOptions.Id.Value+".nuspec")
	}

	pack.VerboseOut(opts.PackageCreateOptions.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", opts.PackageCreateOptions.Id.Value, opts.PackageCreateOptions.Version.Value)
	outFilePath := pack.BuildOutFileName("nupkg", opts.PackageCreateOptions.Id.Value, opts.PackageCreateOptions.Version.Value)

	err = pack.BuildPackage(opts.PackageCreateOptions, outFilePath)
	if err != nil {
		return err
	}

	if nuspecFilePath != "" {
		return os.Remove(nuspecFilePath)
	}

	return nil
}

func PromptMissing(opts *NuPkgCreateOptions) error {
	if len(opts.Author.Value) == 0 {
		for {
			var author string
			if err := opts.Ask(&survey.Input{
				Message: "Author",
				Help:    "Add an author to the package metadata; if no authors are specified the author will default to the current user.",
			}, &author); err != nil {
				return err
			}
			if author == "" {
				break
			}
			opts.Author.Value = append(opts.Author.Value, author)
		}
	}

	if opts.Title.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Nuspec title",
			Help:    "The title to include in the Nuspec file.",
		}, &opts.Title.Value); err != nil {
			return err
		}
	}

	if opts.Description.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Nuspec description",
			Help:    "The description to include in the Nuspec file; defaults to \"A deployment package created from files on disk.\".",
		}, &opts.Description.Value); err != nil {
			return err
		}
	}

	if opts.ReleaseNotes.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Nuspec release notes",
			Help:    "The release notes to include in the Nuspec file.",
		}, &opts.ReleaseNotes.Value); err != nil {
			return err
		}
	}

	if opts.ReleaseNotesFile.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Nuspec release notes file",
			Help:    "A path to a release notes file whose contents will be included in the Nuspec file's release notes.",
		}, &opts.ReleaseNotesFile.Value); err != nil {
			return err
		}
	}

	return nil
}

func applyDefaultsToUnspecifiedPackageOptions(opts *NuPkgCreateOptions) error {
	if opts.PackageCreateOptions.Version.Value == "" {
		opts.PackageCreateOptions.Version.Value = pack.BuildTimestampSemVer(time.Now())
	}

	if opts.PackageCreateOptions.BasePath.Value == "" {
		opts.PackageCreateOptions.BasePath.Value = "."
	}

	if opts.PackageCreateOptions.OutFolder.Value == "" {
		opts.PackageCreateOptions.OutFolder.Value = "."
	}

	if len(opts.PackageCreateOptions.Include.Value) == 0 {
		opts.PackageCreateOptions.Include.Value = append(opts.PackageCreateOptions.Include.Value, "**")
	}

	if opts.Description.Value == "" {
		opts.Description.Value = "A deployment package created from files on disk."
	}

	if util.Empty(opts.Author.Value) {
		currentUser, err := user.Current()
		if err != nil {
			return err
		}
		opts.Author.Value = append(opts.Author.Value, currentUser.Name)
	}

	return nil
}

func getReleaseNotesFromFile(filePath string) (string, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	notes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(notes), nil
}

func shouldGenerateNuSpec(opts *NuPkgCreateOptions) bool {
	return opts.Description.Value != "" ||
		opts.Title.Value != "" ||
		opts.ReleaseNotes.Value != "" ||
		opts.ReleaseNotesFile.Value != "" ||
		!util.Empty(opts.Author.Value)
}

func GenerateNuSpec(opts *NuPkgCreateOptions) (string, error) {
	var err error
	releaseNotes := opts.ReleaseNotes.Value
	if opts.ReleaseNotesFile.Value != "" {
		if releaseNotes != "" {
			return "", errors.New(`cannot specify both "Nuspec release notes" and "Nuspec release notes file"`)
		}
		releaseNotes, err = getReleaseNotesFromFile(opts.ReleaseNotesFile.Value)
		if err != nil {
			return "", err
		}
	}

	filePath := filepath.Join(opts.PackageCreateOptions.BasePath.Value, opts.PackageCreateOptions.Id.Value+".nuspec")

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<package xmlns="http://schemas.microsoft.com/packaging/2010/07/nuspec.xsd">` + "\n")
	sb.WriteString("  <metadata>\n")
	sb.WriteString("    <id>" + opts.PackageCreateOptions.Id.Value + "</id>\n")
	sb.WriteString("    <version>" + opts.PackageCreateOptions.Version.Value + "</version>\n")
	sb.WriteString("    <description>" + opts.Description.Value + "</description>\n")
	sb.WriteString("    <authors>" + strings.Join(opts.Author.Value, ",") + "</authors>\n")
	if releaseNotes != "" {
		sb.WriteString("    <releaseNotes>" + releaseNotes + "</releaseNotes>\n")
	}
	sb.WriteString("  </metadata>\n")
	sb.WriteString("</package>\n")

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}

	_, err = file.WriteString(sb.String())
	if err != nil {
		return "", err
	}

	return filePath, file.Close()
}
