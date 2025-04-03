package create

import (
	"errors"
	"fmt"
	"os"
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
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const (
	FlagAuthor           = "author"
	FlagTitle            = "title"
	FlagDescription      = "description"
	FlagReleaseNotes     = "releaseNotes"
	FlagReleaseNotesFile = "releaseNotesFile"
)

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
			$ %[1]s package nuget create --id SomePackage --version 1.0.0
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

	nuspecFilePath := ""
	if shouldGenerateNuSpec(opts) {
		defer func() {
			if nuspecFilePath != "" {
				defer cleanupFile(nuspecFilePath)
			}
		}()
		nuspecFilePath, err = GenerateNuSpec(opts)
		if err != nil {
			return err
		}
		opts.Include.Value = append(opts.Include.Value, nuspecFilePath)
	}

	contentTypesFilePath, err := generateContentTypesFile(opts)
	if err != nil {
		return err
	}
	defer cleanupFile(contentTypesFilePath)
	opts.Include.Value = append(opts.Include.Value, contentTypesFilePath)

	corePropertiesFilePath, err := generateCorePropertiesFile(opts)
	if err != nil {
		return err
	}
	defer cleanupFile(corePropertiesFilePath)
	opts.Include.Value = append(opts.Include.Value, corePropertiesFilePath)

	relsFilePath, err := generateRelsFile(opts, nuspecFilePath, corePropertiesFilePath)
	if err != nil {
		return err
	}
	defer cleanupFile(relsFilePath)
	opts.Include.Value = append(opts.Include.Value, relsFilePath)

	pack.VerboseOut(opts.Writer, opts.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", opts.Id.Value, opts.Version.Value)
	outFilePath := pack.BuildOutFileName("nupkg", opts.Id.Value, opts.Version.Value)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath,
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
				Default: "A deployment package created from files on disk.",
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

	if len(opts.Author.Value) > 0 {
		if opts.Description.Value == "" {
			opts.Description.Value = "A deployment package created from files on disk."
		}
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

	if opts.Description.Value == "" {
		return "", errors.New("description is required when generating nuspec metadata")
	}
	if len(opts.Author.Value) == 0 {
		return "", errors.New("at least one author is required when generating nuspec metadata")
	}

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

func generateContentTypesFile(opts *NuPkgCreateOptions) (string, error) {
	filePath := filepath.Join(opts.BasePath.Value, "[Content_Types].xml")
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml" />
  <Default Extension="psmdcp" ContentType="application/vnd.openxmlformats-package.core-properties+xml" />
  <Default Extension="json" ContentType="application/octet" />
  <Default Extension="dll" ContentType="application/octet" />
  <Default Extension="pdb" ContentType="application/octet" />
  <Default Extension="exe" ContentType="application/octet" />
  <Default Extension="nuspec" ContentType="application/octet" />
</Types>`)
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

func generateCorePropertiesFile(opts *NuPkgCreateOptions) (string, error) {
	fileName := strings.Replace(uuid.New().String(), "-", "", -1) + ".psmdcp"
	filePath := filepath.Join(opts.BasePath.Value, "package", "services", "metadata", "core-properties", fileName)
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<coreProperties xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://schemas.openxmlformats.org/package/2006/metadata/core-properties">
  <dc:creator>@</dc:creator>
  <dc:description>` + opts.Description.Value + `</dc:description>
  <dc:identifier>` + opts.Id.Value + `</dc:identifier>
  <version>` + opts.Version.Value + `</version>
  <keywords></keywords>
  <lastModifiedBy>Octopus CLI</lastModifiedBy>
</coreProperties>`)
	err := os.MkdirAll(filepath.Dir(filePath), 0770)
	if err != nil {
		return "", err
	}
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

func generateRelsFile(opts *NuPkgCreateOptions, nuspecFilePath, corePropertiesFilePath string) (string, error) {
	filePath := filepath.Join(opts.BasePath.Value, "_rels", ".rels")
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Type="http://schemas.microsoft.com/packaging/2010/07/manifest" Target="/` + filepath.Base(nuspecFilePath) + `" Id="R1" />
  <Relationship Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="/package/services/metadata/core-properties/` +
		filepath.Base(corePropertiesFilePath) + `" Id="R2" />
</Relationships>`)
	err := os.MkdirAll(filepath.Dir(filePath), 0770)
	if err != nil {
		return "", err
	}
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

func cleanupFile(filePath string) {
	err := os.Remove(filePath)
	if err != nil {
		panic(err)
	}
}
