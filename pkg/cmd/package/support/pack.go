package support

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	FlagId        = "id"
	FlagVersion   = "version"
	FlagBasePath  = "base-path"
	FlagOutFolder = "out-folder"
	FlagInclude   = "include"
	FlagExclude   = "exclude"
	FlagVerbose   = "verbose"
	FlagOverwrite = "overwrite"
)

type PackageCreateFlags struct {
	Id        *flag.Flag[string]
	Version   *flag.Flag[string]
	BasePath  *flag.Flag[string]
	OutFolder *flag.Flag[string]
	Include   *flag.Flag[[]string]
	Exclude   *flag.Flag[[]string]
	Verbose   *flag.Flag[bool]
	Overwrite *flag.Flag[bool]
}

type PackageCreateOptions struct {
	*PackageCreateFlags
	Writer   io.Writer
	Ask      question.Asker
	NoPrompt bool
	CmdPath  string
}

func NewPackageCreateFlags() *PackageCreateFlags {
	return &PackageCreateFlags{
		Id:        flag.New[string](FlagId, false),
		Version:   flag.New[string](FlagVersion, false),
		BasePath:  flag.New[string](FlagBasePath, false),
		OutFolder: flag.New[string](FlagOutFolder, false),
		Include:   flag.New[[]string](FlagInclude, false),
		Exclude:   flag.New[[]string](FlagExclude, false),
		Verbose:   flag.New[bool](FlagVerbose, false),
		Overwrite: flag.New[bool](FlagOverwrite, false),
	}
}

func NewPackageCreateOptions(f factory.Factory, flags *PackageCreateFlags, cmd *cobra.Command) *PackageCreateOptions {
	return &PackageCreateOptions{
		PackageCreateFlags: flags,
		Writer:             cmd.OutOrStdout(),
		Ask:                f.Ask,
		NoPrompt:           !f.IsPromptEnabled(),
		CmdPath:            cmd.CommandPath(),
	}
}

func PackageCreatePromptMissing(opts *PackageCreateOptions) error {
	if opts.Id.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Package ID",
			Help:    "The ID of the package.",
		}, &opts.Id.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			survey.MaxLength(200),
			survey.MinLength(1),
		))); err != nil {
			return err
		}
	}

	if opts.Version.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Version",
			Help:    "The version of the package, must be a valid SemVer.",
		}, &opts.Version.Value); err != nil {
			return err
		}
	}

	if opts.BasePath.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Base Path",
			Help:    "Root folder containing the contents to zip; defaults to current directory.",
			Default: ".",
		}, &opts.BasePath.Value); err != nil {
			return err
		}
	}

	if opts.OutFolder.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Out Folder",
			Help:    "Folder into which the zip file will be written; defaults to the current directory.",
			Default: ".",
		}, &opts.OutFolder.Value); err != nil {
			return err
		}
	}

	if len(opts.Include.Value) == 0 {
		for {
			var pattern string
			if err := opts.Ask(&survey.Input{
				Message: "Include patterns",
				Help:    "Add a file pattern to include, relative to the base path e.g. /bin/*.dll; defaults to \"**\" no patterns provided.",
			}, &pattern); err != nil {
				return err
			}
			if pattern == "" {
				break
			}
			opts.Include.Value = append(opts.Include.Value, pattern)
		}
	}

	if len(opts.Exclude.Value) == 0 {
		for {
			var pattern string
			if err := opts.Ask(&survey.Input{
				Message: "Exclude patterns",
				Help:    "Add a file pattern to exclude, relative to the base path e.g. **/*.config; leave blank to exclude nothing.",
			}, &pattern); err != nil {
				return err
			}
			if pattern == "" {
				break
			}
			opts.Exclude.Value = append(opts.Exclude.Value, pattern)
		}
	}

	if !opts.Verbose.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "Verbose",
			Default: true,
		}, &opts.Verbose.Value)
		if err != nil {
			return err
		}
	}

	if !opts.Overwrite.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "Overwrite",
			Default: true,
		}, &opts.Overwrite.Value)
		if err != nil {
			return err
		}
	}

	return nil
}

func VerboseOut(out io.Writer, isVerbose bool, messageTemplate string, messageArgs ...any) {
	if isVerbose {
		fmt.Fprintf(out, messageTemplate, messageArgs...)
	}
}

// PackageCreatedOutput is the JSON shape written after a package is created.
type PackageCreatedOutput struct {
	Path string
}

// PrintPackageCreated writes the path of a newly created package in the
// requested output format. The JSON form is marshalled rather than assembled by
// hand so that paths containing backslashes, such as Windows paths, are escaped
// correctly and the output remains parseable.
func PrintPackageCreated(cmd *cobra.Command, outputFormat string, path string) {
	switch outputFormat {
	case constants.OutputFormatBasic:
		cmd.Printf("%s\n", path)
	case constants.OutputFormatJson:
		data, _ := json.Marshal(PackageCreatedOutput{Path: path})
		cmd.Println(string(data))
	default: // table
		cmd.Printf("Successfully created package %s\n", path)
	}
}

func BuildTimestampSemVer(dateTime time.Time) string {
	endSegment := dateTime.Hour()*10000 + dateTime.Minute()*100 + dateTime.Second()
	return fmt.Sprintf("%d.%d.%d.%d", dateTime.Year(), dateTime.Month(), dateTime.Day(), endSegment)
}

func BuildOutFileName(packageType, id, version string) string {
	return fmt.Sprintf("%s.%s.%s", id, version, packageType)
}

// ArchiveEntry is written into the archive from memory rather than read from
// disk, for parts that are generated rather than packaged.
type ArchiveEntry struct {
	Name    string
	Content []byte
}

// PackageContents adjusts how an archive is assembled, for formats that need
// more than a plain zip of the matched files.
type PackageContents struct {
	// ExtraEntries is given the relative paths already selected for the archive
	// and returns parts to append. Called after the file list is resolved, so a
	// format can describe its own contents.
	ExtraEntries func(paths []string) ([]ArchiveEntry, error)

	// ExcludeDirectories omits explicit directory records. Open Packaging
	// Conventions containers, such as .nupkg, must not contain part names
	// ending in "/".
	ExcludeDirectories bool
}

func BuildPackage(opts *PackageCreateOptions, outFileName string) (*os.File, error) {
	return BuildPackageWithContents(opts, outFileName, nil)
}

func BuildPackageWithContents(opts *PackageCreateOptions, outFileName string, contents *PackageContents) (*os.File, error) {
	outFilePath := filepath.Join(opts.OutFolder.Value, outFileName)
	outPath, err := filepath.Abs(opts.OutFolder.Value)
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(outFilePath)
	if !opts.Overwrite.Value && err == nil {
		return nil, fmt.Errorf("package with name '%s' already exists ...aborting", outFileName)
	}

	VerboseOut(opts.Writer, opts.Verbose.Value, "Saving \"%s\" to \"%s\"...\nAdding files from \"%s\" matching pattern/s \"%s\"\n", outFileName, outPath, opts.BasePath.Value, strings.Join(opts.Include.Value, ", "))
	if len(opts.Exclude.Value) > 0 {
		VerboseOut(opts.Writer, opts.Verbose.Value, "Excluding files matching pattern/s \"%s\"\n", strings.Join(opts.Exclude.Value, ", "))
	}

	filePaths, err := getDistinctPatternMatches(opts.BasePath.Value, opts.Include.Value, opts.Exclude.Value)
	if err != nil {
		return nil, err
	}

	if len(filePaths) == 0 {
		return nil, errors.New("no files identified to package")
	}

	var extraEntries []ArchiveEntry
	if contents != nil {
		// filter before the hook runs, so a format describing its own contents
		// is given the paths that will actually be in the archive
		if contents.ExcludeDirectories {
			filePaths, err = withoutDirectories(opts.BasePath.Value, filePaths)
			if err != nil {
				return nil, err
			}
		}

		if contents.ExtraEntries != nil {
			extraEntries, err = contents.ExtraEntries(filePaths)
			if err != nil {
				return nil, err
			}
		}
	}

	return buildArchive(opts.Writer, outFilePath, opts.BasePath.Value, filePaths, opts.Verbose.Value, extraEntries)
}

// withoutDirectories drops directory entries from a resolved file list. OPC
// containers such as .nupkg must not contain part names ending in "/".
func withoutDirectories(basePath string, paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		fileInfo, err := os.Stat(filepath.Join(basePath, path))
		if err != nil {
			return nil, err
		}

		if !fileInfo.IsDir() {
			files = append(files, path)
		}
	}

	return files, nil
}

func buildArchive(out io.Writer, outFilePath string, basePath string, filesToArchive []string, isVerbose bool, extraEntries []ArchiveEntry) (*os.File, error) {
	_, outFile := filepath.Split(outFilePath)
	zipFile, err := os.Create(outFilePath)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	for _, path := range filesToArchive {
		if path == outFile || path == "." {
			continue
		}

		fullPath := filepath.Join(basePath, path)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return nil, err
		}

		header.Method = zip.Deflate
		header.Name = path
		if fileInfo.IsDir() {
			header.Name += "/"
		}

		headerWriter, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}

		if fileInfo.IsDir() {
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(headerWriter, f)
		if err == nil {
			VerboseOut(out, isVerbose, "Added file: %s\n", path)
		} else {
			return nil, err
		}

		err = f.Close()
		if err != nil {
			return nil, err
		}
	}

	for _, entry := range extraEntries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		header.SetMode(0644)

		headerWriter, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}

		if _, err = headerWriter.Write(entry.Content); err != nil {
			return nil, err
		}

		VerboseOut(out, isVerbose, "Added file: %s\n", entry.Name)
	}

	return zipFile, nil
}

func getDistinctPatternMatches(basePath string, includePatterns []string, excludePatterns []string) ([]string, error) {
	fileSys := os.DirFS(filepath.Clean(basePath))
	var filePaths []string

	for _, pattern := range includePatterns {
		paths, err := doublestar.Glob(fileSys, filepath.ToSlash(pattern))
		if err != nil {
			return nil, err
		}
		filePaths = append(filePaths, paths...)
	}

	filePaths = util.SliceDistinct(filePaths)

	if len(excludePatterns) == 0 {
		return filePaths, nil
	}

	var keptPaths []string
	for _, path := range filePaths {
		excluded, err := matchesAnyPattern(path, excludePatterns)
		if err != nil {
			return nil, err
		}
		if !excluded {
			keptPaths = append(keptPaths, path)
		}
	}

	return keptPaths, nil
}

// matchesAnyPattern reports whether path matches any of the supplied glob
// patterns. A directory matches when the pattern targets its contents (e.g.
// "AppData/**" excludes the AppData directory as well as everything inside it)
// so that excluded directories are not added to the archive as empty entries.
func matchesAnyPattern(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}

		if directoryPrefix, found := strings.CutSuffix(pattern, "/**"); found {
			matched, err = doublestar.Match(directoryPrefix, path)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
	}

	return false, nil
}
