package create

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagAuthor           = "author"
	FlagTitle            = "title"
	FlagDescription      = "description"
	FlagReleaseNotes     = "releaseNotes"
	FlagReleaseNotesFile = "releaseNotesFile"
)

// DefaultDescription is used when the caller supplies no description. The nuspec
// schema requires the element, and this is what the flag help has always
// promised.
const DefaultDescription = "A deployment package created from files on disk."

type NuPkgCreateFlags struct {
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
		Author:           flag.New[[]string](FlagAuthor, false),
		Title:            flag.New[string](FlagTitle, false),
		Description:      flag.New[string](FlagDescription, false),
		ReleaseNotes:     flag.New[string](FlagReleaseNotes, false),
		ReleaseNotesFile: flag.New[string](FlagReleaseNotesFile, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewNuPkgCreateFlags()
	packFlags := pack.NewPackageCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create nuget",
		Long:  "Create nuget package",
		Example: heredoc.Docf(`
			%[1]s package nuget create --id SomePackage --version 1.0.0
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := &NuPkgCreateOptions{
				NuPkgCreateFlags:     createFlags,
				PackageCreateOptions: pack.NewPackageCreateOptions(f, packFlags, cmd),
			}
			return createRun(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&packFlags.Id.Value, packFlags.Id.Name, "", "The ID of the package")
	flags.StringVarP(&packFlags.Version.Value, packFlags.Version.Name, "v", "", "The version of the package, must be a valid SemVer.")
	flags.StringVar(&packFlags.BasePath.Value, packFlags.BasePath.Name, "", "Root folder containing the contents to zip.")
	flags.StringVar(&packFlags.OutFolder.Value, packFlags.OutFolder.Name, "", "Folder into which the zip file will be written.")
	flags.StringSliceVar(&packFlags.Include.Value, packFlags.Include.Name, []string{}, "Add a file pattern to include, relative to the base path e.g. /bin/*.dll; defaults to \"**\".")
	flags.StringSliceVar(&packFlags.Exclude.Value, packFlags.Exclude.Name, []string{}, "Add a file pattern to exclude, relative to the base path e.g. **/*.config; applied after --include.")
	flags.BoolVar(&packFlags.Verbose.Value, packFlags.Verbose.Name, false, "Verbose output.")
	flags.BoolVar(&packFlags.Overwrite.Value, packFlags.Overwrite.Name, false, "Allow an existing package file of the same ID/version to be overwritten.")
	flags.StringSliceVar(&createFlags.Author.Value, createFlags.Author.Name, []string{}, "Add author/s to the package metadata.")
	flags.StringVar(&createFlags.Title.Value, createFlags.Title.Name, "", "The title of the package.")
	flags.StringVar(&createFlags.Description.Value, createFlags.Description.Name, "", "A description of the package, defaults to \"A deployment package created from files on disk.\".")
	flags.StringVar(&createFlags.ReleaseNotes.Value, createFlags.ReleaseNotes.Name, "", "Release notes for this version of the package.")
	flags.StringVar(&createFlags.ReleaseNotesFile.Value, createFlags.ReleaseNotesFile.Name, "", "A file containing release notes for this version of the package.")
	flags.SortFlags = false

	return cmd
}

func createRun(cmd *cobra.Command, opts *NuPkgCreateOptions) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}
	if !opts.NoPrompt {
		if err := pack.PackageCreatePromptMissing(opts.PackageCreateOptions); err != nil {
			return err
		}
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.Id.Value == "" {
		return errors.New("must supply a package ID")
	}

	err = applyDefaultsToUnspecifiedPackageOptions(opts)
	if err != nil {
		return err
	}

	// Every .nupkg needs a manifest: readers locate it by looking for a single
	// .nuspec in the archive root, so a package without one is not a valid NuGet
	// package even though it is a perfectly good zip. Generate one unless the
	// base path already supplies it, in which case that file is authoritative and
	// must not be touched.
	nuspecFileName := opts.Id.Value + ".nuspec"
	suppliedNuSpec, err := hasSuppliedNuSpec(opts.BasePath.Value, nuspecFileName)
	if err != nil {
		return err
	}

	if suppliedNuSpec {
		pack.VerboseOut(opts.Writer, opts.Verbose.Value, "Using existing nuspec file \"%s\"\n", nuspecFileName)
	} else {
		nuspecFilePath, err := GenerateNuSpec(opts)
		if err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(nuspecFilePath); err != nil {
				pack.VerboseOut(opts.Writer, opts.Verbose.Value, "Could not remove generated nuspec file \"%s\": %v\n", nuspecFilePath, err)
			}
		}()
	}

	opts.Include.Value = append(opts.Include.Value, nuspecFileName)

	pack.VerboseOut(opts.Writer, opts.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", opts.Id.Value, opts.Version.Value)
	outFilePath := pack.BuildOutFileName("nupkg", opts.Id.Value, opts.Version.Value)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath, "",
			opts.Author,
			opts.Title,
			opts.Description,
			opts.ReleaseNotes,
			opts.ReleaseNotesFile,
			opts.Id,
			opts.Version,
			opts.BasePath,
			opts.OutFolder,
			opts.Include,
			opts.Exclude,
			opts.Verbose,
			opts.Overwrite,
		)
		fmt.Fprintf(opts.Writer, "\nAutomation Command: %s\n", autoCmd)
	}

	nuget, err := pack.BuildPackage(opts.PackageCreateOptions, outFilePath)
	if nuget != nil {
		switch outputFormat {
		case constants.OutputFormatBasic:
			cmd.Printf("%s\n", nuget.Name())
		case constants.OutputFormatJson:
			cmd.Printf(`{"Path":"%s"}`, nuget.Name())
			cmd.Println()
		default: // table
			cmd.Printf("Successfully created package %s\n", nuget.Name())
		}
	}
	return err
}

func PromptMissing(opts *NuPkgCreateOptions) error {
	if len(opts.Author.Value) == 0 {
		for {
			author := ""
			if err := opts.Ask(&survey.Input{
				Message: "Author (leave blank to continue)",
				Help:    "Add an author to the package metadata.",
			}, &author); err != nil {
				return err
			}
			if strings.TrimSpace(author) == "" {
				break
			}
			opts.Author.Value = append(opts.Author.Value, author)
		}
	}

	if len(opts.Author.Value) > 0 {
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
				Help:    "The description to include in the Nuspec file.",
				Default: DefaultDescription,
			}, &opts.Description.Value); err != nil {
				return err
			}
		}

		if opts.ReleaseNotes.Value == "" {
			if err := opts.Ask(&surveyext.OctoEditor{
				Editor: &survey.Editor{
					Message: "Nuspec release notes",
					Help:    "The release notes to include in the Nuspec file.",
				},
				Optional: true,
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
	}

	return nil
}

func applyDefaultsToUnspecifiedPackageOptions(opts *NuPkgCreateOptions) error {
	if opts.Version.Value == "" {
		opts.Version.Value = pack.BuildTimestampSemVer(time.Now())
	}

	if opts.BasePath.Value == "" {
		opts.BasePath.Value = "."
	}

	if opts.OutFolder.Value == "" {
		opts.OutFolder.Value = "."
	}

	if len(opts.Include.Value) == 0 {
		opts.Include.Value = append(opts.Include.Value, "**")
	}

	// A manifest is generated for every package now, so these two are no longer
	// only relevant when the caller opted into metadata: the nuspec schema
	// requires both, and leaving them out produces a package strict readers
	// reject.
	if opts.Description.Value == "" {
		opts.Description.Value = DefaultDescription
	}

	if util.Empty(opts.Author.Value) {
		opts.Author.Value = []string{defaultAuthor(opts.Id.Value)}
	}

	return nil
}

// defaultAuthor mirrors what the old Octopus CLI did, and what the Author flag
// has always said it should do. Where the user cannot be determined the package
// ID stands in: the element is required, and an obvious placeholder beats
// failing the command over metadata nobody asked for.
// currentUser is a seam: user lookup fails on some minimal container images, and
// the fallback needs to be reachable in a test.
var currentUser = user.Current

func defaultAuthor(fallback string) string {
	if current, err := currentUser(); err == nil {
		if name := strings.TrimSpace(current.Username); name != "" {
			return name
		}
	}
	return fallback
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

// hasSuppliedNuSpec reports whether the base path already contains a manifest
// for this package. One written by hand is the user's own file: it is theirs to
// keep, and generating over the top of it would both discard their metadata and
// delete the file on the way out.
func hasSuppliedNuSpec(basePath string, nuspecFileName string) (bool, error) {
	_, err := os.Stat(filepath.Join(basePath, nuspecFileName))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func GenerateNuSpec(opts *NuPkgCreateOptions) (string, error) {

	releaseNotes := opts.ReleaseNotes.Value
	if opts.ReleaseNotesFile.Value != "" {
		if releaseNotes != "" {
			return "", errors.New(`cannot specify both "Nuspec release notes" and "Nuspec release notes file"`)
		}

		notes, err := getReleaseNotesFromFile(opts.ReleaseNotesFile.Value)
		releaseNotes = notes
		if err != nil {
			return "", err
		}
	}

	filePath := filepath.Join(opts.BasePath.Value, opts.Id.Value+".nuspec")

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<package xmlns="http://schemas.microsoft.com/packaging/2010/07/nuspec.xsd">` + "\n")
	sb.WriteString("  <metadata>\n")
	sb.WriteString("    <id>" + opts.Id.Value + "</id>\n")
	sb.WriteString("    <version>" + opts.Version.Value + "</version>\n")
	if opts.Title.Value != "" {
		sb.WriteString("    <title>" + opts.Title.Value + "</title>\n")
	}
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
